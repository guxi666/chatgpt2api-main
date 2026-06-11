import {
  DEFAULT_CHAT_MODEL,
  createChatCompletionTask,
  fetchCreationTasks,
  type CreationTaskMessage,
  type ImageOutputFormat,
} from "@/lib/api";
import type { ImageSizeSelection } from "@/app/image/image-options";
import type { StoredReferenceImage } from "@/store/image-conversations";

export type EcommerceProductInfo = {
  productName: string;
  productCategory: string;
  appearance: string;
  coreFunction: string;
  sellingPoints: string[];
  targetAudience: string;
  useScenarios: string;
  designStyle: string;
  colorPrimary: string;
  colorSecondary: string;
  colorRelation: string;
  material: string;
  texture: string;
  touchInference: string;
  detail: string;
  craft: string;
  coreFeatures: string;
};

export type EcommerceUploadKind = "material" | "reference";

export const ecommerceSizeOptions = [
  { value: "auto", label: "Auto" },
  { value: "1:1", label: "1:1" },
  { value: "4:5", label: "4:5" },
  { value: "3:4", label: "3:4" },
  { value: "16:9", label: "16:9" },
  { value: "9:16", label: "9:16" },
] as const;

export type EcommerceSizePreset = (typeof ecommerceSizeOptions)[number]["value"];

export type EcommerceGenerateRequest = {
  prompt: string;
  count: number;
  language: string;
  categoryName: string;
  designSpecText: string;
  promptPlans: EcommercePromptPlan[];
  size: string;
  sizeSelection: ImageSizeSelection;
  outputFormat: ImageOutputFormat;
  outputCompression?: number;
  materialImages: StoredReferenceImage[];
  effectReferenceImages: StoredReferenceImage[];
  referenceImages: StoredReferenceImage[];
};

export type EcommercePromptPlan = {
  id: string;
  title: string;
  prompt: string;
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

function normalizeTextValue(value: unknown, fallback = "未识别") {
  const text = String(value || "").trim();
  return text || fallback;
}

function normalizeListValue(value: unknown, minimum = 0) {
  const rawItems = Array.isArray(value)
    ? value
    : String(value || "")
        .split(/\r?\n|；|;|、/)
        .map((item) => item.trim());
  const items = rawItems
    .map((item) => String(item || "").replace(/^[-\d.、\s]+/, "").trim())
    .filter(Boolean);
  while (items.length < minimum) {
    items.push("未识别");
  }
  return items;
}

function extractLineValue(text: string, labels: string[]) {
  for (const label of labels) {
    const pattern = new RegExp(`${label}\\s*[:：]\\s*([^\\n]+)`, "i");
    const match = text.match(pattern);
    if (match?.[1]?.trim()) {
      return match[1].trim();
    }
  }
  return "";
}

function extractBulletBlock(text: string, label: string) {
  const escapedLabel = label.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const match = text.match(new RegExp(`${escapedLabel}\\s*[:：]\\s*([\\s\\S]*?)(?:\\n\\d+\\.|$)`, "i"));
  if (!match?.[1]) {
    return [];
  }
  return normalizeListValue(match[1]);
}

export function buildEmptyEcommerceProductInfo(fillValue: string): EcommerceProductInfo {
  const sellingPointFill = [fillValue, fillValue, fillValue];
  return {
    productName: fillValue,
    productCategory: fillValue,
    appearance: fillValue,
    coreFunction: fillValue,
    sellingPoints: sellingPointFill,
    targetAudience: fillValue,
    useScenarios: fillValue,
    designStyle: fillValue,
    colorPrimary: fillValue,
    colorSecondary: fillValue,
    colorRelation: fillValue,
    material: fillValue,
    texture: fillValue,
    touchInference: fillValue,
    detail: fillValue,
    craft: fillValue,
    coreFeatures: fillValue,
  };
}

export function normalizeEcommerceProductInfo(text: string): EcommerceProductInfo {
  const parsed = extractJsonObject(text);
  if (parsed) {
    const color = parsed.color_palette && typeof parsed.color_palette === "object" ? (parsed.color_palette as Record<string, unknown>) : {};
    const material = parsed.material_texture && typeof parsed.material_texture === "object" ? (parsed.material_texture as Record<string, unknown>) : {};
    const detail = parsed.detail_craft && typeof parsed.detail_craft === "object" ? (parsed.detail_craft as Record<string, unknown>) : {};
    const sellingPoints = normalizeListValue(parsed.selling_points || parsed.product_selling_points || parsed.product_highlights, 3);
    const hasStructuredValue =
      parsed.product_name ||
      parsed.product_category ||
      parsed.appearance ||
      parsed.core_function ||
      parsed.target_audience ||
      parsed.use_scenarios ||
      parsed.design_style ||
      parsed.core_features;
    if (hasStructuredValue || sellingPoints.some((item) => item !== "未识别")) {
      return {
        productName: normalizeTextValue(parsed.product_name || parsed.name),
        productCategory: normalizeTextValue(parsed.product_category || parsed.category),
        appearance: normalizeTextValue(parsed.appearance || parsed.appearance_description),
        coreFunction: normalizeTextValue(parsed.core_function || parsed.core_usage || parsed.usage),
        sellingPoints,
        targetAudience: normalizeTextValue(parsed.target_audience || parsed.target_people),
        useScenarios: normalizeTextValue(parsed.use_scenarios || parsed.scenarios),
        designStyle: normalizeTextValue(parsed.design_style || parsed.style),
        colorPrimary: normalizeTextValue(color.main || color.primary || parsed.color_primary),
        colorSecondary: normalizeTextValue(color.secondary || parsed.color_secondary),
        colorRelation: normalizeTextValue(color.relationship || parsed.color_relation),
        material: normalizeTextValue(material.material || parsed.material),
        texture: normalizeTextValue(material.texture || parsed.texture),
        touchInference: normalizeTextValue(material.touch_inference || material.touch || parsed.touch_inference),
        detail: normalizeTextValue(detail.detail || parsed.detail),
        craft: normalizeTextValue(detail.craft || parsed.craft),
        coreFeatures: normalizeTextValue(parsed.core_features || parsed.feature_summary || parsed.features),
      };
    }
  }

  return {
    productName: normalizeTextValue(extractLineValue(text, ["1\\.\\s*产品名称", "产品名称"])),
    productCategory: normalizeTextValue(extractLineValue(text, ["2\\.\\s*产品品类", "产品品类", "品类"])),
    appearance: normalizeTextValue(extractLineValue(text, ["3\\.\\s*外观直接描述", "外观直接描述"])),
    coreFunction: normalizeTextValue(extractLineValue(text, ["4\\.\\s*核心功能/用途", "核心功能/用途"])),
    sellingPoints: (() => {
      const points = extractBulletBlock(text, "5. 产品卖点");
      return points.length > 0 ? normalizeListValue(points, 3) : ["未识别", "未识别", "未识别"];
    })(),
    targetAudience: normalizeTextValue(extractLineValue(text, ["6\\.\\s*目标人群", "目标人群"])),
    useScenarios: normalizeTextValue(extractLineValue(text, ["7\\.\\s*使用场景", "使用场景"])),
    designStyle: normalizeTextValue(extractLineValue(text, ["8\\.\\s*设计风格", "设计风格"])),
    colorPrimary: normalizeTextValue(extractLineValue(text, ["主色"])),
    colorSecondary: normalizeTextValue(extractLineValue(text, ["辅助色"])),
    colorRelation: normalizeTextValue(extractLineValue(text, ["色彩关系"])),
    material: normalizeTextValue(extractLineValue(text, ["材质"])),
    texture: normalizeTextValue(extractLineValue(text, ["质感"])),
    touchInference: normalizeTextValue(extractLineValue(text, ["触感推测"])),
    detail: normalizeTextValue(extractLineValue(text, ["细节"])),
    craft: normalizeTextValue(extractLineValue(text, ["工艺"])),
    coreFeatures: normalizeTextValue(extractLineValue(text, ["12\\.\\s*核心特征", "核心特征"])),
  };
}

function buildAnalyzePrompt(categoryName: string) {
  return [
    "你是电商商品信息识别助手。请根据用户上传的素材图片识别商品信息。",
    "只输出 JSON，不要输出 Markdown、解释或多余文字。",
    "JSON 字段固定为：",
    '{"product_name":"产品名称","product_category":"产品品类","appearance":"外观直接描述","core_function":"核心功能/用途","selling_points":["产品卖点1","产品卖点2","产品卖点3"],"target_audience":"目标人群","use_scenarios":"使用场景","design_style":"设计风格","color_palette":{"main":"主色","secondary":"辅助色","relationship":"色彩关系"},"material_texture":{"material":"材质","texture":"质感","touch_inference":"触感推测"},"detail_craft":{"detail":"细节","craft":"工艺"},"core_features":"核心特征"}',
    "其中 selling_points 至少返回 3 条，且严格锚定图片可见信息或可合理确认的产品属性。",
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

export function buildEcommerceProductTemplate(info: EcommerceProductInfo) {
  return [
    `1. 产品名称：${info.productName}`,
    `2. 产品品类：${info.productCategory}`,
    `3. 外观直接描述：${info.appearance}`,
    `4. 核心功能/用途：${info.coreFunction}`,
    "5. 产品卖点：",
    ...info.sellingPoints.map((item) => `- ${item}`),
    `6. 目标人群：${info.targetAudience}`,
    `7. 使用场景：${info.useScenarios}`,
    `8. 设计风格：${info.designStyle}`,
    "9. 色彩搭配：",
    `- 主色：${info.colorPrimary}`,
    `- 辅助色：${info.colorSecondary}`,
    `- 色彩关系：${info.colorRelation}`,
    "10. 材质与质感：",
    `- 材质：${info.material}`,
    `- 质感：${info.texture}`,
    `- 触感推测：${info.touchInference}`,
    "11. 细节与工艺：",
    `- 细节：${info.detail}`,
    `- 工艺：${info.craft}`,
    `12. 核心特征：${info.coreFeatures}`,
  ].join("\n");
}

function productSubjectLabel(info: EcommerceProductInfo) {
  return `${info.productName}${info.productCategory !== "未识别" ? `（${info.productCategory}）` : ""}`;
}

function pickFallback(value: string, fallback: string) {
  return value && value !== "未识别" ? value : fallback;
}

export function buildEcommerceDesignSpecText(info: EcommerceProductInfo) {
  const subject = productSubjectLabel(info);
  const style = pickFallback(info.designStyle, "高端极简商业电商风格");
  const primary = pickFallback(info.colorPrimary, "#1A2A47（稳重深色系）");
  const secondary = pickFallback(info.colorSecondary, "#D4AF37（高级金属点缀）");
  const accent = info.material !== "未识别" ? "#8E8E8E（呼应材质质感）" : "#8E8E8E（中性质感灰）";
  const background = info.appearance !== "未识别" ? "#F7F7F7（干净浅灰背景）" : "#F7F7F7（纯净暖灰背景）";
  const material = pickFallback(info.material, "亲肤材质与高质感复合材料");
  const texture = pickFallback(info.texture, "细腻柔和、具备商业广告级质感");
  const touch = pickFallback(info.touchInference, "舒适、轻盈、不压迫");
  const detail = pickFallback(info.detail, "重点展示结构细节、功能分区与边缘处理");
  const craft = pickFallback(info.craft, "精致拼接、圆润收边与专业级产品工艺");
  const featureFocus = pickFallback(info.coreFeatures, info.coreFunction);

  return [
    "所有图片必须遵循以下统一规范，确保视觉连贯性。",
    "",
    "视觉风格",
    `风格定位：${style}，突出${subject}的专业感与购买说服力。`,
    "色彩系统",
    `主色调：${primary}；`,
    `辅助色：${secondary}；`,
    `点缀色：${accent}；`,
    `背景色：${background}。`,
    "字体系统",
    "标题字体：思源黑体 Bold；",
    "正文字体：思源黑体 Regular；",
    "字号层级：大标题:副标题:正文=3:1.8:1。",
    "视觉语言",
    `装饰元素：围绕${subject}加入极简几何线条、柔和光晕、信息标签与结构化卖点模块；`,
    "图标风格：线性细线，简洁精致；",
    "留白原则：保持 35% 以上边缘留白，突出主体并保留电商排版呼吸感。",
    "摄影风格",
    `光线：柔和侧逆光，重点表现${material}与${texture}；`,
    "景深：中浅景深，聚焦产品主体与关键细节；",
    `相机参数参考：f/8.0, 1/125s, ISO 100, 85mm，重点强调${featureFocus}。`,
    "品质要求",
    "分辨率：4K/高清；",
    "风格：专业产品摄影/商业广告级；",
    "真实感：超写实/照片级。",
    "",
    "材质与质感补充",
    `材质：${material}`,
    `质感：${texture}`,
    `触感推测：${touch}`,
    "",
    "细节与工艺补充",
    `细节：${detail}`,
    `工艺：${craft}`,
  ].join("\n");
}

const ECOMMERCE_PROMPT_PLAN_PRESETS = [
  { title: "产品展示：主视觉主图", prompt: "突出产品主体，建立第一眼视觉识别与核心卖点认知。" },
  { title: "卖点聚焦：核心功能展示", prompt: "重点放大核心功能与关键卖点，用信息标签强化购买理由。" },
  { title: "细节特写：材质与工艺", prompt: "聚焦局部细节、材质肌理与做工表现，提升品质感。" },
  { title: "场景展示：办公使用场景", prompt: "在办公或桌面场景中展示产品使用氛围，强化代入感。" },
  { title: "场景展示：居家舒适场景", prompt: "在居家氛围中强调轻松、舒适、日常使用体验。" },
  { title: "人群沟通：目标用户共鸣", prompt: "围绕目标人群痛点与收益进行视觉表达，突出情绪价值。" },
  { title: "功能分解：信息模块海报", prompt: "以结构化排版拆解多个功能点，适合详情页信息展示。" },
  { title: "组合展示：产品与配件", prompt: "展示产品本体与相关组成、包装或配件，提升完整感。" },
  { title: "多场景延展：通勤/出差使用", prompt: "强调便携性、续航或多场景适配能力，体现使用自由度。" },
  { title: "收官海报：高级品牌感", prompt: "以更强的品牌氛围和商业质感做收束，适合作为结尾视觉。" },
] as const;

export function buildEcommercePromptPlans(info: EcommerceProductInfo, count: number) {
  const subject = productSubjectLabel(info);
  const sellingPoints = normalizeListValue(info.sellingPoints, 3);
  const appearance = pickFallback(info.appearance, "产品外观特征");
  const usage = pickFallback(info.coreFunction, "核心功能");
  const audience = pickFallback(info.targetAudience, "目标消费人群");
  const scene = pickFallback(info.useScenarios, "典型使用场景");
  const featureSummary = pickFallback(info.coreFeatures, usage);
  const max = Math.max(1, Math.min(10, count));

  return ECOMMERCE_PROMPT_PLAN_PRESETS.slice(0, max).map((preset, index) => ({
    id: `plan-${index + 1}`,
    title: preset.title,
    prompt: [
      `本张图片主题：${preset.title}。`,
      `主体产品：${subject}。`,
      `视觉重点：${preset.prompt}`,
      `产品外观：${appearance}。`,
      `核心功能：${usage}。`,
      `产品卖点：${sellingPoints.slice(0, 3).join("；")}。`,
      `目标人群：${audience}。`,
      `使用场景：${scene}。`,
      `核心特征：${featureSummary}。`,
    ].join("\n"),
  }));
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
