"use client";

import { Check, ChevronDown, Globe2, Image as ImageIcon, LoaderCircle, PackageSearch, RotateCcw, X } from "lucide-react";
import { useMemo, useState } from "react";

import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover";
import {
  EFFECT_REFERENCE_IMAGE_LIMIT,
  MATERIAL_IMAGE_LIMIT,
  ecommerceLanguageOptions,
  type EcommerceLanguage,
  type EcommerceProductInfo,
} from "@/app/image/ecommerce-utils";
import { cn } from "@/lib/utils";
import type { StoredReferenceImage } from "@/store/image-conversations";

type EcommerceComposerPanelProps = {
  materialImages: StoredReferenceImage[];
  effectReferenceImages: StoredReferenceImage[];
  productInfo: EcommerceProductInfo | null;
  language: EcommerceLanguage;
  count: number;
  imageCountLimit: number;
  isAnalyzing: boolean;
  analyzeError: string;
  onClearMaterialImages: () => void;
  onClearEffectReferenceImages: () => void;
  onReanalyze: () => void;
  onLanguageChange: (language: EcommerceLanguage) => void;
  onCountChange: (count: number) => void;
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
        <span className="truncate text-xs text-[#8e8e93]">点击右侧图片按钮上传</span>
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
  imageCountLimit,
  isAnalyzing,
  analyzeError,
  onClearMaterialImages,
  onClearEffectReferenceImages,
  onReanalyze,
  onLanguageChange,
  onCountChange,
}: EcommerceComposerPanelProps) {
  const [isQuantityOpen, setIsQuantityOpen] = useState(false);
  const availableQuantities = useMemo(() => quantityOptions(imageCountLimit), [imageCountLimit]);
  const productInfoStatus = productInfo
    ? analyzeError
      ? "识别没有完成，已把可编辑模板放到下方输入框。"
      : "产品信息已写入下方输入框，可直接编辑自定义文案。"
    : "上传素材图片后，产品信息会自动写入下方输入框。";

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

        <div className="grid grid-cols-2 gap-2 sm:w-[22rem]">
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
                className="w-[min(calc(100vw-2rem),20rem)] rounded-[18px] border-[#e5e7eb] bg-white p-3 shadow-[0_24px_80px_-32px_rgba(15,23,42,0.35)]"
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
                          setIsQuantityOpen(false);
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
              </PopoverContent>
            </Popover>
          </div>
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

      {materialImages.length > 0 && !isAnalyzing ? (
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
