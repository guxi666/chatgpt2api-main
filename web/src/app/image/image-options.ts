export const IMAGE_SIZE_OPTIONS = [
  { value: "", label: "Auto" },
  { value: "1:1", label: "1:1 (正方形)" },
  { value: "3:2", label: "3:2 (横版)" },
  { value: "16:9", label: "16:9 (横版)" },
  { value: "4:3", label: "4:3 (横版)" },
  { value: "3:4", label: "3:4 (竖版)" },
  { value: "9:16", label: "9:16 (竖版)" },
] as const;

export const IMAGE_QUALITY_OPTIONS = [
  { value: "low", label: "Low", description: "低质量，速度更快，适合草稿测试" },
  { value: "medium", label: "Medium", description: "均衡质量与速度，适合日常生成" },
  { value: "high", label: "High", description: "高质量，耗时更长，适合最终出图" },
] as const;
