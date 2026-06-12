"use client";

import { BadgePercent, Gift, Home as HomeIcon, LoaderCircle, MessageSquarePlus, ShoppingCart, Trash2 } from "lucide-react";

import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";
import { getImageConversationStats, type ImageConversation } from "@/store/image-conversations";

type ImageSidebarProps = {
  conversations: ImageConversation[];
  isLoadingHistory: boolean;
  selectedConversationId: string | null;
  onOpenHome?: () => void;
  onCreateDraft: () => void;
  onCreateEcommerceDraft?: () => void;
  onClearHistory: () => void | Promise<void>;
  onSelectConversation: (id: string) => void;
  onOpenEcommerce?: () => void;
  isEcommerceActive?: boolean;
  onDeleteConversation: (id: string) => void | Promise<void>;
  formatConversationTime: (value: string) => string;
  hideActionButtons?: boolean;
};

export function ImageSidebar({
  conversations,
  isLoadingHistory,
  selectedConversationId,
  onOpenHome,
  onCreateDraft,
  onCreateEcommerceDraft,
  onClearHistory,
  onSelectConversation,
  onOpenEcommerce,
  isEcommerceActive = false,
  onDeleteConversation,
  formatConversationTime,
  hideActionButtons = false,
}: ImageSidebarProps) {
  return (
    <aside className="relative h-full min-h-0 overflow-hidden">
      <div className="flex h-full min-h-0 flex-col gap-2 py-1 sm:gap-3 sm:py-2">
        {!hideActionButtons && (
          <div className="relative z-10 flex flex-col gap-2">
            <Button
              className="h-10 w-full justify-center gap-2 rounded-full bg-[#181e25] text-white shadow-sm hover:bg-[#2a323d]"
              onClick={onOpenHome || onCreateDraft}
            >
              <HomeIcon className="size-4" />
              首页
            </Button>
            <Button
              className="h-10 w-full justify-center gap-2 rounded-full bg-[#181e25] text-white shadow-sm hover:bg-[#2a323d]"
              onClick={onCreateDraft}
            >
              <MessageSquarePlus className="size-4" />
              新建对话
            </Button>
            {onOpenEcommerce ? (
              <Button
                variant="outline"
                className={cn(
                  "h-10 w-full justify-center gap-2 rounded-full",
                  isEcommerceActive
                    ? "border-[#f2c3a3] bg-[#fff0e4] text-[#d7651f] hover:bg-[#fff0e4] hover:text-[#d7651f]"
                    : "border-[#f2c3a3] bg-[#fff7f1] text-[#d7651f] hover:bg-[#fff0e4] hover:text-[#d7651f]",
                )}
                onClick={onOpenEcommerce}
              >
                <ShoppingCart className="size-4" />
                电商专区
              </Button>
            ) : null}
            {onCreateEcommerceDraft ? (
              <Button
                variant="outline"
                className="h-10 w-full justify-center gap-2 rounded-full border-[#f2c3a3] bg-[#fff0e4] text-[#d7651f] hover:bg-[#fff0e4] hover:text-[#d7651f]"
                onClick={onCreateEcommerceDraft}
              >
                <ShoppingCart className="size-4" />
                新建电商窗口
              </Button>
            ) : null}
            <div className="flex justify-end">
              <Button
                variant="outline"
                className="h-9 rounded-full border-[#e5e7eb] bg-white px-3 text-[#45515e] hover:bg-black/[0.05]"
                onClick={() => void onClearHistory()}
                disabled={conversations.length === 0}
                aria-label="清空历史记录"
                title="清空历史记录"
              >
                <Trash2 className="size-4" />
              </Button>
            </div>
          </div>
        )}

        {isEcommerceActive && !hideActionButtons ? (
          <div className="pointer-events-none absolute inset-x-0 bottom-1 z-0 hidden h-[18rem] overflow-hidden sm:block">
            <div className="absolute bottom-0 left-[-18%] h-16 w-[135%] rotate-[-4deg] rounded-[50%] bg-[#ffd8bf]/55 blur-2xl" />
            <ShoppingCart className="absolute bottom-10 left-1/2 size-32 -translate-x-1/2 text-[#ffb98f]/50 drop-shadow-[0_28px_34px_rgba(247,137,82,0.24)]" strokeWidth={1.35} />
            <span className="absolute bottom-[9.6rem] left-[42%] inline-flex size-10 rotate-[-10deg] items-center justify-center rounded-[14px] bg-[#ffd0ad]/65 text-[#ff8a45] shadow-[0_16px_40px_-28px_rgba(236,105,49,0.8)]">
              <Gift className="size-5" />
            </span>
            <span className="absolute bottom-[7.2rem] right-[16%] inline-flex size-9 rotate-[18deg] items-center justify-center rounded-[14px] bg-[#ffe2cf]/70 text-[#ff9a58] shadow-[0_16px_40px_-28px_rgba(236,105,49,0.8)]">
              <BadgePercent className="size-[18px]" />
            </span>
          </div>
        ) : null}

        <div
          className={cn(
            "relative z-10 min-h-0 flex-1 overflow-y-auto [scrollbar-color:rgba(142,142,147,.45)_transparent] [scrollbar-width:thin] [&::-webkit-scrollbar]:w-1.5 [&::-webkit-scrollbar-thumb]:rounded-full [&::-webkit-scrollbar-thumb]:bg-[#8e8e93]/45 [&::-webkit-scrollbar-track]:bg-transparent",
            hideActionButtons ? "flex flex-col gap-1 pr-0" : "flex flex-col gap-2 pr-1",
          )}
        >
          {isLoadingHistory ? (
            <div className="flex items-center gap-2 px-2 py-3 text-sm text-stone-500">
              <LoaderCircle className="size-4 animate-spin" />
              正在读取会话记录
            </div>
          ) : conversations.length === 0 ? (
            <div className="px-2 py-3 text-sm leading-6 text-stone-500">还没有图片记录，输入提示词后会在这里显示。</div>
          ) : (
            conversations.map((conversation) => {
              const active = conversation.id === selectedConversationId;
              const stats = getImageConversationStats(conversation);
              return (
                <div
                  key={conversation.id}
                  className={cn(
                    "group relative w-full rounded-[16px] border text-left transition",
                    hideActionButtons ? "px-4 py-3.5" : "px-3 py-2 sm:py-3",
                    active
                      ? "border-[#f2f3f5] bg-white text-[#18181b] shadow-[0_4px_6px_rgba(0,0,0,0.08)]"
                      : "border-transparent text-[#45515e] hover:border-[#f2f3f5] hover:bg-white",
                  )}
                >
                  <button
                    type="button"
                    onClick={() => onSelectConversation(conversation.id)}
                    className={cn("block w-full text-left", hideActionButtons ? "pr-0" : "pr-8")}
                  >
                    <div className={cn("truncate font-semibold", hideActionButtons ? "text-base" : "text-sm")}>
                      <span className="truncate">{conversation.title}</span>
                    </div>
                    <div className={cn("mt-1 text-xs", active ? "text-[#45515e]" : "text-[#8e8e93]")}>
                      {conversation.turns.length} 轮 · {formatConversationTime(conversation.updatedAt)}
                    </div>
                    {stats.running > 0 || stats.queued > 0 ? (
                      <div className="mt-2 flex flex-wrap items-center gap-2 text-[11px]">
                        {stats.running > 0 ? (
                          <span className="rounded-full bg-blue-50 px-2 py-1 text-blue-600">处理中 {stats.running}</span>
                        ) : null}
                        {stats.queued > 0 ? (
                          <span className="rounded-full bg-amber-50 px-2 py-1 text-amber-700">排队 {stats.queued}</span>
                        ) : null}
                      </div>
                    ) : null}
                  </button>
                  {!hideActionButtons ? (
                    <button
                      type="button"
                      onClick={() => void onDeleteConversation(conversation.id)}
                      className="absolute top-3 right-2 inline-flex size-7 items-center justify-center rounded-md text-stone-400 opacity-0 transition hover:bg-stone-100 hover:text-rose-500 group-hover:opacity-100"
                      aria-label="删除会话"
                    >
                      <Trash2 className="size-4" />
                    </button>
                  ) : null}
                </div>
              );
            })
          )}
        </div>
      </div>
    </aside>
  );
}
