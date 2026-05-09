"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Check, Copy, Download, Eye, ImageIcon, LoaderCircle, RefreshCw, Search, Trash2 } from "lucide-react";
import { toast } from "sonner";

import { DateRangeFilter } from "@/components/date-range-filter";
import { ImageLightbox } from "@/components/image-lightbox";
import { PageHeader } from "@/components/page-header";
import { Button } from "@/components/ui/button";
import { Card, CardContent } from "@/components/ui/card";
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { deleteManagedImages, fetchManagedImages, type ManagedImage } from "@/lib/api";
import { formatImageFileSize } from "@/lib/image-size";
import { useAuthGuard } from "@/lib/use-auth-guard";

const PAGE_SIZE_OPTIONS = [10, 20, 50, 100] as const;

type DeleteImageTarget = {
  paths: string[];
};

function managedImageKey(item: ManagedImage) {
  return item.path;
}

function buildManagedImageDownloadName(item: ManagedImage, index: number) {
  const sourceName = item.name || item.url.split("?")[0]?.split("/").filter(Boolean).pop();
  if (sourceName) return sourceName;
  return `managed-image-${String(index + 1).padStart(2, "0")}.png`;
}

async function downloadManagedImage(item: ManagedImage, index: number) {
  let href = item.url;
  let objectUrl = "";

  try {
    const response = await fetch(item.url);
    if (response.ok) {
      const blob = await response.blob();
      objectUrl = URL.createObjectURL(blob);
      href = objectUrl;
    }
  } catch {
    href = item.url;
  }

  const link = document.createElement("a");
  link.href = href;
  link.download = buildManagedImageDownloadName(item, index);
  document.body.appendChild(link);
  link.click();
  link.remove();

  if (objectUrl) window.setTimeout(() => URL.revokeObjectURL(objectUrl), 1000);
}

function sleep(ms: number) {
  return new Promise((resolve) => window.setTimeout(resolve, ms));
}

function isRequestCanceled(error: unknown) {
  return error instanceof Error && error.message === "canceled";
}

function getManagedImageFormatLabel(item: ManagedImage) {
  const normalized = (item.name || item.url).split("?")[0]?.match(/\.([a-z0-9]+)$/i)?.[1] || "image";
  const format = normalized.toLowerCase() === "jpeg" ? "jpg" : normalized.toLowerCase();
  return format.toUpperCase();
}

function ImageManagerContent({ canDeleteImages }: { canDeleteImages: boolean }) {
  const activeLoadRef = useRef<AbortController | null>(null);
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [items, setItems] = useState<ManagedImage[]>([]);
  const [selectedImageIds, setSelectedImageIds] = useState<Record<string, boolean>>({});
  const [downloadingKey, setDownloadingKey] = useState<string | null>(null);
  const [deleteTarget, setDeleteTarget] = useState<DeleteImageTarget | null>(null);
  const [isDeleting, setIsDeleting] = useState(false);
  const [lightboxIndex, setLightboxIndex] = useState(0);
  const [lightboxOpen, setLightboxOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [pageSize, setPageSize] = useState<number>(20);
  const [page, setPage] = useState(1);

  const lightboxImages = useMemo(
    () =>
      items.map((item) => ({
        id: item.path,
        src: item.url,
        sizeLabel: formatImageFileSize(item.size),
        dimensions: item.width && item.height ? `${item.width} x ${item.height}` : undefined,
      })),
    [items],
  );

  const selectedItems = useMemo(
    () => items.filter((item) => selectedImageIds[managedImageKey(item)]),
    [items, selectedImageIds],
  );
  const selectedCount = selectedItems.length;
  const allSelected = items.length > 0 && selectedCount === items.length;
  const isMutatingImages = downloadingKey !== null || isDeleting;

  const totalPages = Math.max(1, Math.ceil(items.length / pageSize));
  const currentPage = Math.min(page, totalPages);

  useEffect(() => {
    setPage((prev) => Math.max(1, Math.min(prev, totalPages)));
  }, [totalPages]);

  const pagedItems = useMemo(() => {
    const start = (currentPage - 1) * pageSize;
    return items.slice(start, start + pageSize);
  }, [currentPage, items, pageSize]);

  const loadImages = useCallback(async () => {
    activeLoadRef.current?.abort();
    const controller = new AbortController();
    activeLoadRef.current = controller;
    setIsLoading(true);
    setLoadError("");

    try {
      const data = await fetchManagedImages(
        { start_date: startDate, end_date: endDate },
        { signal: controller.signal },
      );
      setItems(data.items);
      setSelectedImageIds({});
      setPage(1);
    } catch (error) {
      if (controller.signal.aborted || isRequestCanceled(error)) return;
      const message = error instanceof Error ? error.message : "加载图片失败";
      setLoadError(message);
      toast.error(message);
    } finally {
      if (activeLoadRef.current === controller) {
        activeLoadRef.current = null;
        setIsLoading(false);
      }
    }
  }, [endDate, startDate]);

  useEffect(() => {
    void loadImages();
  }, [loadImages]);

  useEffect(() => () => activeLoadRef.current?.abort(), []);

  const clearFilters = () => {
    setStartDate("");
    setEndDate("");
  };

  const toggleImageSelection = (item: ManagedImage) => {
    const key = managedImageKey(item);
    setSelectedImageIds((current) => ({
      ...current,
      [key]: !current[key],
    }));
  };

  const toggleAllImages = () => {
    if (allSelected) {
      setSelectedImageIds({});
      return;
    }
    setSelectedImageIds(Object.fromEntries(items.map((item) => [managedImageKey(item), true])));
  };

  const downloadItems = async (key: string, targetItems: ManagedImage[]) => {
    if (targetItems.length === 0 || downloadingKey) return;
    setDownloadingKey(key);
    try {
      for (let index = 0; index < targetItems.length; index += 1) {
        const item = targetItems[index];
        await downloadManagedImage(item, items.indexOf(item));
        if (index < targetItems.length - 1) await sleep(120);
      }
    } finally {
      setDownloadingKey(null);
    }
  };

  const openDeleteConfirm = (targetItems: ManagedImage[]) => {
    if (!canDeleteImages) return;
    const paths = Array.from(new Set(targetItems.map((item) => item.path)));
    if (paths.length === 0) {
      toast.error("没有可删除的图片");
      return;
    }
    setDeleteTarget({ paths });
  };

  const handleConfirmDelete = async () => {
    if (!canDeleteImages || !deleteTarget || isDeleting) return;
    const paths = deleteTarget.paths;
    const pathSet = new Set(paths);
    setIsDeleting(true);
    try {
      const data = await deleteManagedImages(paths);
      setItems((current) => current.filter((item) => !pathSet.has(item.path)));
      setSelectedImageIds((current) => {
        const next = { ...current };
        paths.forEach((path) => delete next[path]);
        return next;
      });
      setLightboxOpen(false);
      setLightboxIndex(0);
      setDeleteTarget(null);
      toast.success(
        data.missing > 0
          ? `已删除 ${data.deleted} 张，${data.missing} 张不存在`
          : `已删除 ${data.deleted} 张图片`,
      );
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "删除图片失败");
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <section className="flex flex-col gap-5">
      <PageHeader
        eyebrow="Images"
        title="图片库"
        actions={(
          <>
            <DateRangeFilter
              startDate={startDate}
              endDate={endDate}
              onChange={(start, end) => {
                setStartDate(start);
                setEndDate(end);
              }}
            />
            <Button variant="outline" onClick={clearFilters} className="h-10 rounded-lg">
              清除筛选
            </Button>
            <Button onClick={() => void loadImages()} disabled={isLoading || isMutatingImages} className="h-10 rounded-lg">
              {isLoading ? <LoaderCircle className="size-4 animate-spin" /> : <Search className="size-4" />}
              查询
            </Button>
          </>
        )}
      />

      <Card className="overflow-hidden rounded-[20px]">
        <CardContent className="p-0">
          <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-5 py-4">
            <div className="flex items-center gap-2 text-sm text-muted-foreground">
              <ImageIcon className="size-4" />
              共 {items.length} 张
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm text-muted-foreground">每页</span>
              <Select
                value={String(pageSize)}
                onValueChange={(value: string) => {
                  setPageSize(Number(value));
                  setPage(1);
                }}
              >
                <SelectTrigger className="h-8 w-[96px] rounded-lg">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {PAGE_SIZE_OPTIONS.map((size) => (
                    <SelectItem key={size} value={String(size)}>{size}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
              <Button
                type="button"
                variant="outline"
                className="h-8 rounded-lg px-3 text-xs"
                disabled={items.length === 0 || isMutatingImages}
                onClick={toggleAllImages}
              >
                {allSelected ? "取消全选" : "全选"}
              </Button>
              <Button
                type="button"
                className="h-8 rounded-lg px-2.5 text-[11px]"
                disabled={selectedCount === 0 || isMutatingImages}
                onClick={() => void downloadItems("selected", selectedItems)}
              >
                {downloadingKey === "selected" ? <LoaderCircle className="size-3 animate-spin" /> : <Download className="size-3" />}
                下载已选 ({selectedCount})
              </Button>
              {canDeleteImages ? (
                <Button
                  type="button"
                  variant="outline"
                  className="h-8 rounded-lg px-2.5 text-[11px] text-rose-600 hover:bg-rose-50 hover:text-rose-700"
                  disabled={selectedCount === 0 || isMutatingImages}
                  onClick={() => openDeleteConfirm(selectedItems)}
                >
                  <Trash2 className="size-3" />
                  删除已选 ({selectedCount})
                </Button>
              ) : null}
              <Button
                type="button"
                variant="outline"
                className="h-8 rounded-lg px-2.5 text-[11px]"
                disabled={items.length === 0 || isMutatingImages}
                onClick={() => void downloadItems("all", items)}
              >
                {downloadingKey === "all" ? <LoaderCircle className="size-3 animate-spin" /> : <Download className="size-3" />}
                下载全部
              </Button>
              <Button variant="outline" className="h-8 rounded-lg px-3 text-xs" onClick={() => void loadImages()} disabled={isLoading || isMutatingImages}>
                <RefreshCw className={`size-4 ${isLoading ? "animate-spin" : ""}`} />
                刷新
              </Button>
            </div>
          </div>

          {isLoading ? (
            <div className="flex min-h-[260px] items-center justify-center">
              <LoaderCircle className="size-6 animate-spin text-stone-400" />
            </div>
          ) : null}

          {!isLoading && loadError ? (
            <div className="px-6 py-14 text-center text-sm text-rose-600">{loadError}</div>
          ) : null}

          {!isLoading && !loadError && items.length === 0 ? (
            <div className="px-6 py-14 text-center text-sm text-muted-foreground">暂无图片</div>
          ) : null}

          {!isLoading && !loadError && pagedItems.length > 0 ? (
            <div className="grid gap-3 px-5 py-4 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-4">
              {pagedItems.map((item) => {
                const globalIndex = items.findIndex((candidate) => candidate.path === item.path);
                const selected = Boolean(selectedImageIds[managedImageKey(item)]);
                const dimensions = item.width && item.height ? `${item.width} x ${item.height}` : "";
                const sizeLabel = formatImageFileSize(item.size);
                const imageMeta = [dimensions, sizeLabel].filter(Boolean).join(" | ");

                return (
                  <figure key={item.path} className={`group relative overflow-hidden rounded-xl border bg-muted/20 ${selected ? "ring-2 ring-[#1456f0]/80 ring-offset-2" : ""}`}>
                    <button
                      type="button"
                      onClick={() => toggleImageSelection(item)}
                      className="block w-full text-left"
                      aria-label={selected ? "取消选择图片" : "选择图片"}
                    >
                      <img
                        src={item.thumbnail_url || item.url}
                        alt={item.name || item.path}
                        width={item.width || undefined}
                        height={item.height || undefined}
                        loading="lazy"
                        decoding="async"
                        className="block h-52 w-full object-cover"
                      />
                    </button>
                    <button
                      type="button"
                      onClick={() => toggleImageSelection(item)}
                      className={`absolute left-2 top-2 z-10 inline-flex size-6 items-center justify-center rounded-full border ${
                        selected
                          ? "border-[#1456f0] bg-[#1456f0] text-white"
                          : "border-white/90 bg-black/25 text-transparent"
                      }`}
                      aria-label={selected ? "取消选择图片" : "选择图片"}
                    >
                      {selected ? <Check className="size-3.5" /> : null}
                    </button>
                    <div className="absolute right-2 top-2 z-10 flex items-center gap-1">
                      <button
                        type="button"
                        onClick={() => {
                          setLightboxIndex(globalIndex);
                          setLightboxOpen(true);
                        }}
                        className="inline-flex h-7 items-center gap-1 rounded-full bg-white/95 px-2 text-[11px] font-medium text-stone-800"
                        aria-label="查看原图"
                        title="查看原图"
                      >
                        <Eye className="size-3" />
                        原图
                      </button>
                      <button
                        type="button"
                        onClick={() => {
                          void navigator.clipboard.writeText(item.url);
                          toast.success("图片地址已复制");
                        }}
                        className="inline-flex size-7 items-center justify-center rounded-full bg-white/95 text-stone-800"
                        aria-label="复制地址"
                        title="复制地址"
                      >
                        <Copy className="size-3.5" />
                      </button>
                      {canDeleteImages ? (
                        <button
                          type="button"
                          onClick={() => openDeleteConfirm([item])}
                          disabled={isDeleting}
                          className="inline-flex size-7 items-center justify-center rounded-full bg-white/95 text-rose-600 disabled:opacity-60"
                          aria-label="删除图片"
                          title="删除图片"
                        >
                          {isDeleting && deleteTarget?.paths.includes(item.path) ? <LoaderCircle className="size-3.5 animate-spin" /> : <Trash2 className="size-3.5" />}
                        </button>
                      ) : null}
                    </div>
                    <figcaption className="space-y-1 px-3 py-2 text-xs">
                      <div className="truncate font-medium text-foreground">{item.name || item.path}</div>
                      <div className="truncate text-muted-foreground">{item.created_at}</div>
                      <div className="truncate text-muted-foreground">{getManagedImageFormatLabel(item)} {imageMeta ? `| ${imageMeta}` : ""}</div>
                    </figcaption>
                  </figure>
                );
              })}
            </div>
          ) : null}

          {!isLoading && !loadError && items.length > 0 ? (
            <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border px-5 py-3 text-sm">
              <span>第 {currentPage} / {totalPages} 页</span>
              <div className="flex items-center gap-2">
                <Button
                  type="button"
                  variant="outline"
                  className="h-8 rounded-lg px-3"
                  disabled={currentPage <= 1}
                  onClick={() => setPage((p) => Math.max(1, p - 1))}
                >
                  上一页
                </Button>
                <Button
                  type="button"
                  variant="outline"
                  className="h-8 rounded-lg px-3"
                  disabled={currentPage >= totalPages}
                  onClick={() => setPage((p) => Math.min(totalPages, p + 1))}
                >
                  下一页
                </Button>
              </div>
            </div>
          ) : null}
        </CardContent>
      </Card>

      <ImageLightbox
        images={lightboxImages}
        currentIndex={lightboxIndex}
        open={lightboxOpen}
        onOpenChange={setLightboxOpen}
        onIndexChange={setLightboxIndex}
      />

      {canDeleteImages && deleteTarget ? (
        <Dialog open onOpenChange={(open) => (!open && !isDeleting ? setDeleteTarget(null) : null)}>
          <DialogContent showCloseButton={false} className="rounded-2xl p-6">
            <DialogHeader className="gap-2">
              <DialogTitle>删除图片</DialogTitle>
              <DialogDescription className="text-sm leading-6">
                确认删除 {deleteTarget.paths.length} 张图片吗？删除后不可恢复。
              </DialogDescription>
            </DialogHeader>
            <DialogFooter>
              <Button type="button" variant="outline" className="h-10 rounded-xl px-5" onClick={() => setDeleteTarget(null)} disabled={isDeleting}>
                取消
              </Button>
              <Button type="button" className="h-10 rounded-xl bg-rose-600 px-5 text-white hover:bg-rose-700" onClick={() => void handleConfirmDelete()} disabled={isDeleting}>
                {isDeleting ? <LoaderCircle className="size-4 animate-spin" /> : <Trash2 className="size-4" />}
                确认删除
              </Button>
            </DialogFooter>
          </DialogContent>
        </Dialog>
      ) : null}
    </section>
  );
}

export default function ImageManagerPage() {
  const { isCheckingAuth, session } = useAuthGuard(["admin", "user"]);
  if (isCheckingAuth || !session) {
    return (
      <div className="flex min-h-[40vh] items-center justify-center">
        <LoaderCircle className="size-5 animate-spin text-stone-400" />
      </div>
    );
  }
  const canDeleteImages = session.role === "admin" || session.provider !== "linuxdo";
  return <ImageManagerContent canDeleteImages={canDeleteImages} />;
}
