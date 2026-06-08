import {
  DEFAULT_CHAT_MODEL,
  createChatCompletionTask,
  fetchCreationTasks,
  type CreationTaskMessage,
} from "@/lib/api";
import type { StoredReferenceImage } from "@/store/image-conversations";

export type EcommerceProductInfo = {
  nameCategory: string;
  features: string;
};

export type EcommerceUploadKind = "material" | "reference";

export type EcommerceGenerateRequest = {
  prompt: string;
  count: number;
  language: string;
  categoryName: string;
  materialImages: StoredReferenceImage[];
  effectReferenceImages: StoredReferenceImage[];
  referenceImages: StoredReferenceImage[];
};

export const MATERIAL_IMAGE_LIMIT = 10;
export const EFFECT_REFERENCE_IMAGE_LIMIT = 1;
export const MAX_UPLOAD_IMAGE_SIZE = 15 * 1024 * 1024;
export const ECOMMERCE_CATEGORY_NAME = "电商效果图";
export const ECOMMERCE_IMAGE_ACCEPT = "image/png,image/jpeg,image/webp";

const ANALYZE_POLL_LIMIT = 90;
const ANALYZE_POLL_INTERVAL = 1500;
const IMAGE_FILE_EXTENSION_PATTERN = /\.(jpeg|jpg|png|webp)$/i;
const IMAGE_MIME_TYPES = new Set(["image/jpeg", "image/png", "image/webp"]);

export const ecommerceLanguageOptions = [
  "简体中文",
  "繁体中文",
  "英语",
  "泰语",
  "俄语",
  "越南语",
  "马来语",
  "葡萄牙语",
  "西班牙语",
  "法语",
  "德语",
  "日语",
  "韩语",
  "意大利语",
  "阿拉伯语",
  "印尼语",
  "菲律宾语",
] as const;

export type EcommerceLanguage = (typeof ecommerceLanguageOptions)[number];

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function createId() {
  if (typeof crypto !== "undefined" && "randomUUID" in crypto) {
    return crypto.randomUUID();
  }
  return `${Date.now()}-${Math.random().toString(16).slice(2)}`;
}

export function isEcommerceUploadImage(file: File) {
  return IMAGE_MIME_TYPES.has(file.type) || IMAGE_FILE_EXTENSION_PATTERN.test(file.name);
}

export function isValidEcommerceUploadImage(file: File) {
  return isEcommerceUploadImage(file) && file.size <= MAX_UPLOAD_IMAGE_SIZE;
}

export function getEcommerceImageFiles(files: FileList | File[]) {
  return Array.from(files).filter(isEcommerceUploadImage);
}

function readFileAsDataUrl(file: File) {
  return new Promise<string>((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(String(reader.result || ""));
    reader.onerror = () => reject(new Error("读取图片失败"));
    reader.readAsDataURL(file);
  });
}

export async function buildEcommerceReferenceImages(files: File[], limit: number) {
  const out: StoredReferenceImage[] = [];
  for (const file of files.slice(0, limit)) {
    out.push({
      name: file.name || "product.png",
      type: file.type || "image/png",
      dataUrl: await readFileAsDataUrl(file),
      source: "upload",
    });
  }
  return out;
}

export function ecommerceUploadKindLabel(kind: EcommerceUploadKind) {
  return kind === "material" ? "素材图片" : "参考图片";
}

export function ecommerceUploadKindLimit(kind: EcommerceUploadKind) {
  return kind === "material" ? MATERIAL_IMAGE_LIMIT : EFFECT_REFERENCE_IMAGE_LIMIT;
}

function extractTaskText(task: Awaited<ReturnType<typeof fetchCreationTasks>>["items"][number]) {
  return task.data?.map((item) => item.text_response || "").find((text) => text.trim())?.trim() || "";
}

function extractJsonObject(text: string) {
  const trimmed = text.trim();
  if (!trimmed) {
    return null;
  }
  try {
    return JSON.parse(trimmed) as Record<string, unknown>;
  } catch {
    const match = trimmed.match(/\{[\s\S]*\}/);
    if (!match) {
      return null;
    }
    try {
      return JSON.parse(match[0]) as Record<string, unknown>;
    } catch {
      return null;
    }
  }
}

export function normalizeEcommerceProductInfo(text: string): EcommerceProductInfo {
  const parsed = extractJsonObject(text);
  if (parsed) {
    const nameCategory = String(parsed.product_name_category || parsed.name_category || parsed.product || "").trim();
    const rawFeatures = parsed.core_features || parsed.features || parsed.feature_list;
    const features = Array.isArray(rawFeatures)
      ? rawFeatures.map((item) => String(item).trim()).filter(Boolean).join("；")
      : String(rawFeatures || "").trim();
    if (nameCategory || features) {
      return {
        nameCategory: nameCategory || "未识别",
        features: features || "未识别",
      };
    }
  }

  const nameMatch = text.match(/(?:产品名称及品类|产品名称|品类|name_category)\s*[:：]\s*([^\n]+)/i);
  const featureMatch = text.match(/(?:核心特征清单|核心特征|features?)\s*[:：]\s*([\s\S]+)/i);
  return {
    nameCategory: nameMatch?.[1]?.replace(/[【】[\]]/g, "").trim() || "未识别",
    features:
      featureMatch?.[1]
        ?.split(/\n/)
        .map((line) => line.replace(/^[-\d.、\s]+/, "").replace(/[【】[\]]/g, "").trim())
        .filter(Boolean)
        .slice(0, 8)
        .join("；") || "未识别",
  };
}

function buildAnalyzePrompt(categoryName: string) {
  return [
    "你是电商商品信息识别助手。请根据用户上传的素材图片识别商品信息。",
    "只输出 JSON，不要输出 Markdown、解释或多余文字。",
    "JSON 字段固定为：",
    '{"product_name_category":"产品名称及品类","core_features":["严格锚定图片可见信息的特征1","特征2","特征3"]}',
    "要求：不要虚构图片中看不清的品牌、材质、型号或功效；无法确认时写“未识别”。",
    `用户选择的电商方向：${categoryName}。`,
  ].join("\n");
}

async function waitForTextTask(taskId: string) {
  for (let index = 0; index < ANALYZE_POLL_LIMIT; index += 1) {
    const taskList = await fetchCreationTasks([taskId]);
    const task = taskList.items[0];
    if (!task) {
      await sleep(ANALYZE_POLL_INTERVAL);
      continue;
    }
    if (task.status === "success") {
      const text = extractTaskText(task);
      if (text) {
        return text;
      }
      throw new Error("模型没有返回产品识别内容");
    }
    if (task.status === "error" || task.status === "cancelled") {
      throw new Error(task.error || "产品识别任务失败");
    }
    await sleep(ANALYZE_POLL_INTERVAL);
  }
  throw new Error("产品识别超时，请稍后重试");
}

export async function analyzeEcommerceProductImages(images: StoredReferenceImage[], categoryName = ECOMMERCE_CATEGORY_NAME) {
  const prompt = buildAnalyzePrompt(categoryName);
  const messages: CreationTaskMessage[] = [
    { role: "system", content: "你只做商品图片识别，并严格返回 JSON。" },
    { role: "user", content: prompt },
  ];
  const task = await createChatCompletionTask(
    `ecommerce-analyze-${createId()}`,
    prompt,
    DEFAULT_CHAT_MODEL,
    messages,
    images.map((image) => image.dataUrl),
  );
  const text = await waitForTextTask(task.id);
  return normalizeEcommerceProductInfo(text);
}

export function buildEcommerceProductTemplate(info: EcommerceProductInfo, customCopy: string) {
  return [
    "【产品信息】",
    `1、产品名称及品类：【${info.nameCategory}】`,
    `2、核心特征清单（严格锚定）：【${info.features}】`,
    `3、自定义文案：【${customCopy.trim()}】`,
  ].join("\n");
}

export function extractEcommerceCustomCopy(prompt: string) {
  const match = prompt.match(/3、\s*自定义文案：\s*【([\s\S]*?)】/);
  if (match) {
    return match[1].trim();
  }
  const trimmed = prompt.trim();
  return trimmed.startsWith("【产品信息】") ? "" : trimmed;
}

export function buildEcommerceGenerationPromptFromPrompt({
  prompt,
  language,
}: {
  prompt: string;
  language: EcommerceLanguage;
}) {
  return [
    prompt.trim(),
    "",
    "请根据以上产品信息和素材图片生成电商效果图。",
    `画面中的标题、卖点短句、标签和所有可见文案必须使用${language}。`,
    "素材图片是用户自己的产品，请严格保留素材图中的产品外观、颜色、结构、材质和可见细节，不要改变产品主体，不要添加不存在的品牌标识。",
    "参考图片是别人产品的效果图，只学习它的构图、背景、光影、陈列方式、版式节奏和商业质感；不要复制参考图里的产品、品牌、文字或商标。",
    "构图要求：电商主图质感，干净高级的商业布景，产品主体清晰突出，光影自然，留出适合平台展示的视觉呼吸空间。",
  ].join("\n");
}
