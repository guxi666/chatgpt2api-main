"use client";

import { Check, ChevronDown, FileType, Globe2, Image as ImageIcon, LoaderCircle, PackageSearch, PencilLine, RotateCcw, Ruler, Sparkles, Trash2, X } from "lucide-react";
import { useEffect, useMemo, useRef, useState } from "react";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import {
  EFFECT_REFERENCE_IMAGE_LIMIT,
  MATERIAL_IMAGE_LIMIT,
  ecommerceLanguageOptions,
  ecommerceSizeOptions,
  type EcommerceLanguage,
  type EcommercePromptPlan,
  type EcommerceProductInfo,
  type EcommerceSizePreset,
} from "@/app/image/ecommerce-utils";
import { IMAGE_OUTPUT_FORMAT_OPTIONS, type ImageOutputFormat } from "@/lib/api";
import { cn } from "@/lib/utils";
import type { StoredReferenceImage } from "@/store/image-conversations";
import { Textarea } from "@/components/ui/textarea";

type EcommerceComposerPanelProps = {
  materialImages: StoredReferenceImage[];
  effectReferenceImages: StoredReferenceImage[];
  productInfo: EcommerceProductInfo | null;
  language: EcommerceLanguage;
  count: number;
  sizePreset: EcommerceSizePreset;
  outputFormat: ImageOutputFormat;
  designSpecText: string;
  promptPlans: EcommercePromptPlan[];
  imageCountLimit: number;
  isAnalyzing: boolean;
  analyzeError: string;
  onClearMaterialImages: () => void;
  onClearEffectReferenceImages: () => void;
  onReanalyze: () => void;
  onLanguageChange: (language: EcommerceLanguage) => void;
  onCountChange: (count: number) => void;
  onSizePresetChange: (sizePreset: EcommerceSizePreset) => void;
  onOutputFormatChange: (format: ImageOutputFormat) => void;
  onDesignSpecChange: (value: string) => void;
  onPromptPlanSave: (planId: string, nextPlan: Pick<EcommercePromptPlan, "title" | "prompt">) => void;
  onPromptPlanDelete: (planId: string) => void;
};

function quantityOptions(limit: number) {
  const max = Math.max(1, Math.min(10, Math.floor(limit || 1)));
  return Array.from({ length: max }, (_, index) => index + 1);
}

function UploadPreviewStrip({
  label,
  images,
  limit,
  onClear,
}: {
  label: string;
  images: StoredReferenceImage[];
  limit: number;
  onClear: () => void;
}) {
  return (
    <div className="flex min-w-0 items-center gap-2">
      <span className="shrink-0 text-xs font-semibold text-[#45515e]">
        {label} {images.length}/{limit}
      </span>
      {images.length > 0 ? (
        <>
          <div className="hide-scrollbar flex min-w-0 gap-1 overflow-x-auto">
            {images.slice(0, 6).map((image, index) => (
              <div key={`${image.name}-${index}`} className="size-8 shrink-0 overflow-hidden rounded-lg border border-[#e5e7eb] bg-white">
                <img src={image.dataUrl} alt={image.name || label} className="h-full w-full object-cover" />
              </div>
            ))}
          </div>
          <button
            type="button"
            className="inline-flex size-7 shrink-0 items-center justify-center rounded-full text-[#8e8e93] transition hover:bg-black/[0.05] hover:text-[#181e25]"
            onClick={onClear}
            aria-label={`清空${label}`}
            title={`清空${label}`}
          >
            <X className="size-3.5" />
          </button>
        </>
      ) : (
        <span className="truncate text-xs text-[#8e8e93]">点击下方图片按钮上传</span>
      )}
    </div>
  );
}

export function EcommerceComposerPanel({
  materialImages,
  effectReferenceImages,
  productInfo,
  language,
  count,
  sizePreset,
  outputFormat,
  designSpecText,
  promptPlans,
  imageCountLimit,
  isAnalyzing,
  analyzeError,
  onClearMaterialImages,
  onClearEffectReferenceImages,
  onReanalyze,
  onLanguageChange,
  onCountChange,
  onSizePresetChange,
  onOutputFormatChange,
  onDesignSpecChange,
  onPromptPlanSave,
  onPromptPlanDelete,
}: EcommerceComposerPanelProps) {
  const [isQuantityOpen, setIsQuantityOpen] = useState(false);
  const [isSpecExpanded, setIsSpecExpanded] = useState(false);
  const [expandedPlanIds, setExpandedPlanIds] = useState<string[]>([]);
  const [editingPlanId, setEditingPlanId] = useState("");
  const [editingPlanTitle, setEditingPlanTitle] = useState("");
  const [editingPlanPrompt, setEditingPlanPrompt] = useState("");
  const planScrollRef = useRef<HTMLDivElement | null>(null);
  const availableQuantities = useMemo(() => quantityOptions(imageCountLimit), [imageCountLimit]);
  const editingPlan = promptPlans.find((item) => item.id === editingPlanId) ?? null;
  const productInfoStatus = productInfo
    ? analyzeError
      ? "识别没有完成，已把可编辑模板放到下方输入框。"
      : "产品信息已写入下方输入框，可直接编辑自定义文案。"
    : "上传素材图片后，点击 AI帮写 才会写入下方输入框。";

  useEffect(() => {
    if (!editingPlan) {
      setEditingPlanId("");
      return;
    }
    setEditingPlanTitle(editingPlan.title);
    setEditingPlanPrompt(editingPlan.prompt);
  }, [editingPlan]);

  useEffect(() => {
    if (!isQuantityOpen || !planScrollRef.current) {
      return;
    }
    planScrollRef.current.scrollTop = 0;
  }, [count, isQuantityOpen]);

  useEffect(() => {
    if (!isQuantityOpen) {
      return;
    }
    setExpandedPlanIds([]);
  }, [count, isQuantityOpen]);

  return (
    <div className="grid gap-2 border-b border-[#f2f3f5] px-1 pb-3">
      <div className="grid gap-2 lg:grid-cols-[minmax(0,1fr)_auto] lg:items-start">
        <div className="min-w-0">
          <div className="flex items-center gap-2 text-xs font-bold text-[#181e25]">
            <PackageSearch className="size-4 text-[#1456f0]" />
            产品信息
            {isAnalyzing ? (
              <span className="inline-flex items-center gap-1 rounded-full bg-[#f8fafc] px-2 py-0.5 text-[11px] font-semibold text-[#45515e]">
                <LoaderCircle className="size-3 animate-spin" />
                识别中
              </span>
            ) : null}
          </div>
          <div className="mt-1 max-h-24 overflow-y-auto whitespace-pre-wrap text-xs leading-5 text-[#45515e]">
            {productInfoStatus}
          </div>
          {analyzeError ? (
            <div className="mt-1 rounded-lg bg-[#fff7ed] px-2 py-1 text-xs leading-5 text-[#b45309]">
              识别失败：{analyzeError}。可以点击重新识别，或上传更清晰的素材图片。
            </div>
          ) : null}
        </div>

        <div className="grid grid-cols-2 gap-2 sm:w-[32rem] xl:grid-cols-4">
          <label className="grid gap-1 text-[11px] font-semibold text-[#45515e]">
            <span className="inline-flex items-center gap-1">
              <Globe2 className="size-3.5" />
              语言
            </span>
            <select
              value={language}
              onChange={(event) => onLanguageChange(event.target.value as EcommerceLanguage)}
              className="h-9 min-w-0 rounded-full border border-[#e5e7eb] bg-white px-3 text-xs font-semibold text-[#181e25] outline-none transition focus:border-[#1456f0]"
            >
              {ecommerceLanguageOptions.map((item) => (
                <option key={item} value={item}>
                  {item}
                </option>
              ))}
            </select>
          </label>

          <div className="grid gap-1 text-[11px] font-semibold text-[#45515e]">
            <span className="inline-flex items-center gap-1">
              <ImageIcon className="size-3.5" />
              数量
            </span>
            <Popover open={isQuantityOpen} onOpenChange={setIsQuantityOpen}>
              <PopoverTrigger asChild>
                <button
                  type="button"
                  className="flex h-9 items-center justify-between rounded-full border border-[#e5e7eb] bg-white px-3 text-xs font-semibold text-[#181e25] transition hover:border-[#bfdbfe]"
                >
                  <span>{count} 张</span>
                  <ChevronDown className={cn("size-4 transition", isQuantityOpen && "rotate-180")} />
                </button>
              </PopoverTrigger>
              <PopoverContent
                align="end"
                side="top"
                className="w-[min(calc(100vw-2rem),38rem)] rounded-[20px] border-[#e5e7eb] bg-white p-3 shadow-[0_24px_80px_-32px_rgba(15,23,42,0.35)]"
                onOpenAutoFocus={(event) => event.preventDefault()}
              >
                <div className="grid grid-cols-5 gap-2">
                  {availableQuantities.map((item) => {
                    const active = item === count;
                    return (
                      <button
                        key={item}
                        type="button"
                        className={cn(
                          "relative flex h-10 items-center justify-center rounded-lg border text-sm font-bold transition",
                          active
                            ? "border-[#1456f0] text-[#1456f0] ring-1 ring-[#1456f0]"
                            : "border-[#e5e7eb] text-[#686b73] hover:border-[#bfdbfe] hover:text-[#1456f0]",
                        )}
                        onClick={() => {
                          onCountChange(item);
                        }}
                      >
                        {item}
                        {active ? (
                          <span className="absolute bottom-0 right-0 flex size-4 items-center justify-center rounded-tl-md bg-[#1456f0] text-white">
                            <Check className="size-2.5" />
                          </span>
                        ) : null}
                      </button>
                    );
                  })}
                </div>

                <div ref={planScrollRef} className="mt-4 max-h-[min(68vh,34rem)] space-y-3 overflow-y-auto pr-1">
                  <div className="overflow-hidden rounded-[18px] border border-[#e8ebf0] bg-[#f8fafe]">
                    <button
                      type="button"
                      className="flex w-full items-center justify-between gap-3 px-4 py-2.5 text-left"
                      onClick={() => setIsSpecExpanded((current) => !current)}
                    >
                      <span className="inline-flex items-center gap-2 text-sm font-semibold text-[#1f2937]">
                        <span className="inline-flex size-7 items-center justify-center rounded-full bg-white text-[#315efb] shadow-sm">
                          <Sparkles className="size-4" />
                        </span>
                        整体设计规范
                      </span>
                      <ChevronDown className={cn("size-4 text-[#6b7280] transition", isSpecExpanded && "rotate-180")} />
                    </button>
                    {isSpecExpanded ? (
                      <div className="border-t border-[#e8ebf0] bg-white px-4 py-3">
                        <Textarea
                          value={designSpecText}
                          onChange={(event) => onDesignSpecChange(event.target.value)}
                          placeholder="点击 AI帮写 后自动生成整体设计规范"
                          className="min-h-[11rem] resize-none border-0 bg-transparent px-0 py-0 text-sm leading-6 text-[#334155] shadow-none focus-visible:ring-0"
                        />
                      </div>
                    ) : null}
                  </div>

                  <div className="space-y-3">
                    {promptPlans.length > 0 ? (
                      promptPlans.map((plan, index) => {
                        const isEditing = editingPlanId === plan.id;
                        const isExpanded = expandedPlanIds.includes(plan.id);
                        return (
                          <div
                            key={plan.id}
                            className="group rounded-[16px] border border-[#e8ebf0] bg-white p-2.5 shadow-[0_10px_24px_-20px_rgba(15,23,42,0.28)]"
                          >
                            <div className="flex items-start justify-between gap-3">
                              <div className="min-w-0">
                                <button
                                  type="button"
                                  className="flex items-center gap-2 text-left"
                                  onClick={() =>
                                    setExpandedPlanIds((current) =>
                                      current.includes(plan.id)
                                        ? current.filter((item) => item !== plan.id)
                                        : [...current, plan.id],
                                    )
                                  }
                                >
                                  <span className="inline-flex size-6 items-center justify-center rounded-full bg-[#eef3ff] text-xs font-bold text-[#315efb]">
                                    {index + 1}
                                  </span>
                                  <div className="min-w-0">
                                    <div className="truncate text-[13px] font-semibold text-[#111827]">
                                      {isEditing ? editingPlanTitle || plan.title : plan.title}
                                    </div>
                                  </div>
                                  <ChevronDown className={cn("size-4 shrink-0 text-[#94a3b8] transition", isExpanded && "rotate-180")} />
                                </button>
                              </div>
                              <div className="flex items-center gap-1 opacity-100 sm:opacity-0 sm:transition sm:group-hover:opacity-100">
                                <button
                                  type="button"
                                  className="inline-flex size-8 items-center justify-center rounded-full text-[#6b7280] transition hover:bg-[#f3f4f6] hover:text-[#111827]"
                                  onClick={() => {
                                    setEditingPlanId(plan.id);
                                    setEditingPlanTitle(plan.title);
                                    setEditingPlanPrompt(plan.prompt);
                                  }}
                                  aria-label="编辑提示词"
                                  title="编辑提示词"
                                >
                                  <PencilLine className="size-4" />
                                </button>
                                <button
                                  type="button"
                                  className="inline-flex size-8 items-center justify-center rounded-full text-[#6b7280] transition hover:bg-[#fff1f2] hover:text-[#e11d48]"
                                  onClick={() => onPromptPlanDelete(plan.id)}
                                  aria-label="删除提示词"
                                  title="删除提示词"
                                >
                                  <Trash2 className="size-4" />
                                </button>
                              </div>
                            </div>

                            {isEditing ? (
                              <div className="mt-3 space-y-3">
                                <input
                                  value={editingPlanTitle}
                                  onChange={(event) => setEditingPlanTitle(event.target.value)}
                                  className="h-9 w-full rounded-xl border border-[#dbe2ea] px-3 text-sm font-medium text-[#111827] outline-none focus:border-[#315efb]"
                                  placeholder="输入卡片标题"
                                />
                                <Textarea
                                  value={editingPlanPrompt}
                                  onChange={(event) => setEditingPlanPrompt(event.target.value)}
                                  className="min-h-[7.5rem] resize-none rounded-xl border border-[#dbe2ea] text-sm leading-6 text-[#334155] focus-visible:ring-[#315efb]/20"
                                />
                                <div className="flex justify-end gap-2">
                                  <button
                                    type="button"
                                    className="inline-flex h-9 items-center rounded-full border border-[#e5e7eb] px-4 text-sm font-semibold text-[#475569] transition hover:bg-[#f8fafc]"
                                    onClick={() => {
                                      setEditingPlanId("");
                                      setEditingPlanTitle("");
                                      setEditingPlanPrompt("");
                                    }}
                                  >
                                    取消
                                  </button>
                                  <button
                                    type="button"
                                    className="inline-flex h-9 items-center rounded-full bg-[#315efb] px-4 text-sm font-semibold text-white transition hover:bg-[#244bdb]"
                                    onClick={() => {
                                      onPromptPlanSave(plan.id, {
                                        title: editingPlanTitle.trim() || plan.title,
                                        prompt: editingPlanPrompt.trim() || plan.prompt,
                                      });
                                      setEditingPlanId("");
                                    }}
                                  >
                                    保存
                                  </button>
                                </div>
                              </div>
                            ) : isExpanded ? (
                              <div className="mt-2 rounded-[12px] border border-[#edf1f5] bg-[#fbfcfe] px-3 py-2 text-[13px] leading-5 text-[#334155]">
                                <div className="whitespace-pre-wrap">
                                  {plan.prompt}
                                </div>
                              </div>
                            ) : null}
                          </div>
                        );
                      })
                    ) : (
                      <div className="rounded-[18px] border border-dashed border-[#dbe2ea] bg-[#fbfcfe] px-4 py-6 text-center text-sm text-[#64748b]">
                        点击 AI帮写 后自动生成整体规范与对应张数的分图提示词。
                      </div>
                    )}
                  </div>
                </div>
              </PopoverContent>
            </Popover>
          </div>

          <label className="grid gap-1 text-[11px] font-semibold text-[#45515e]">
            <span className="inline-flex items-center gap-1">
              <Ruler className="size-3.5" />
              尺寸
            </span>
            <select
              value={sizePreset}
              onChange={(event) => onSizePresetChange(event.target.value as EcommerceSizePreset)}
              className="h-9 min-w-0 rounded-full border border-[#e5e7eb] bg-white px-3 text-xs font-semibold text-[#181e25] outline-none transition focus:border-[#1456f0]"
            >
              {ecommerceSizeOptions.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </label>

          <label className="grid gap-1 text-[11px] font-semibold text-[#45515e]">
            <span className="inline-flex items-center gap-1">
              <FileType className="size-3.5" />
              格式
            </span>
            <select
              value={outputFormat}
              onChange={(event) => onOutputFormatChange(event.target.value as ImageOutputFormat)}
              className="h-9 min-w-0 rounded-full border border-[#e5e7eb] bg-white px-3 text-xs font-semibold text-[#181e25] outline-none transition focus:border-[#1456f0]"
            >
              {IMAGE_OUTPUT_FORMAT_OPTIONS.map((item) => (
                <option key={item.value} value={item.value}>
                  {item.label}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>

      <div className="grid gap-2 lg:grid-cols-2">
        <UploadPreviewStrip label="素材图片" images={materialImages} limit={MATERIAL_IMAGE_LIMIT} onClear={onClearMaterialImages} />
        <UploadPreviewStrip
          label="参考图片（可选）"
          images={effectReferenceImages}
          limit={EFFECT_REFERENCE_IMAGE_LIMIT}
          onClear={onClearEffectReferenceImages}
        />
      </div>

      {materialImages.length > 0 && productInfo && !isAnalyzing ? (
        <div className="flex justify-end">
          <button
            type="button"
            className="inline-flex h-8 items-center gap-1.5 rounded-full border border-[#e5e7eb] bg-white px-3 text-xs font-semibold text-[#45515e] transition hover:border-[#bfdbfe] hover:text-[#1456f0]"
            onClick={onReanalyze}
          >
            <RotateCcw className="size-3.5" />
            重新识别
          </button>
        </div>
      ) : null}
    </div>
  );
}
