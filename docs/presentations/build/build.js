// 文化官方案 v4 · PPTX 生成脚本
// 12 张幻灯片 · 深色金调 · 严格字号层次
const pptxgen = require("pptxgenjs");

const pres = new pptxgen();
pres.layout = "LAYOUT_WIDE"; // 13.3 x 7.5
pres.title = "文化官方案 v4";
pres.author = "Culture Points Mall";

// ============ 色板 ============
const C = {
  bg: "0A1331",
  bgCard: "15214A",
  bgCardSoft: "1A2855",
  border: "2A3666",
  borderSoft: "20305C",
  gold: "F2B544",
  goldDeep: "DB9C2F",
  goldSoft: "3B2D14",
  cyan: "5BD9CC",
  cyanSoft: "133835",
  pink: "F177B3",
  pinkSoft: "3A1D32",
  red: "F26E72",
  green: "6DE0A4",
  purple: "B49AFF",
  white: "FFFFFF",
  light: "D5DEFF",
  muted: "8595BD",
  mutedDim: "5F6E94",
};

const FONT = "Calibri";

// ============ 字号层级（关键约束） ============
const T = {
  coverTitle: 60,    // 封面主标
  coverSub: 36,      // 封面副标
  coverTag: 16,
  pageTitle: 30,     // 页面大标题
  pageSub: 13,       // 页面副标 / 引导
  eyebrow: 10,       // 章节眼眉
  cardTitle: 15,     // 卡片标题
  cardBody: 12,      // 卡片正文
  bigNum: 54,        // 大数字
  bigNumLabel: 10,
  body: 12,
  caption: 9,
  pill: 9,
  quote: 22,
  quoteSmall: 16,
  tableHead: 11,
  tableBody: 12,
};

// ============ 通用辅助 ============
function bg(slide) { slide.background = { color: C.bg }; }

function makeShadow() {
  return { type: "outer", blur: 14, offset: 3, angle: 90, color: "000000", opacity: 0.30 };
}

function card(slide, { x, y, w, h, fill = C.bgCard, border = C.border, shadow = true }) {
  slide.addShape(pres.shapes.RECTANGLE, {
    x, y, w, h,
    fill: { color: fill },
    line: { color: border, width: 0.75 },
    ...(shadow ? { shadow: makeShadow() } : {}),
  });
}

function eyebrow(slide, text, y = 0.5) {
  slide.addText(text, {
    x: 0.7, y, w: 9, h: 0.3,
    fontSize: T.eyebrow, fontFace: FONT, color: C.gold,
    bold: true, charSpacing: 6, margin: 0,
  });
}

function pageTitle(slide, text, opts = {}) {
  slide.addText(text, {
    x: 0.7, y: 0.85, w: 11.5, h: 0.7,
    fontSize: T.pageTitle, fontFace: FONT, color: C.white,
    bold: true, margin: 0, ...opts,
  });
}

function pageSub(slide, text) {
  slide.addText(text, {
    x: 0.7, y: 1.55, w: 11.5, h: 0.35,
    fontSize: T.pageSub, fontFace: FONT, color: C.muted, margin: 0,
  });
}

function tag(slide, x, y, label, color) {
  // 计算宽度（粗略）
  const w = Math.max(0.7, label.length * 0.13 + 0.3);
  slide.addShape(pres.shapes.ROUNDED_RECTANGLE, {
    x, y, w, h: 0.32,
    fill: { color: C.bg },
    line: { color, width: 0.75 },
    rectRadius: 0.16,
  });
  slide.addText(label, {
    x, y, w, h: 0.32, fontSize: T.pill, fontFace: FONT,
    color, bold: true, align: "center", valign: "middle", margin: 0,
  });
  return w;
}

function footer(slide, pageNo, total = 12) {
  slide.addText("Culture Points Mall · v4 · 项目方案", {
    x: 0.7, y: 7.15, w: 6, h: 0.25, fontSize: 9,
    color: C.mutedDim, charSpacing: 2, fontFace: FONT, margin: 0,
  });
  slide.addText([
    { text: String(pageNo).padStart(2, "0"), options: { color: C.gold, bold: true } },
    { text: ` / ${String(total).padStart(2, "0")}`, options: { color: C.muted } },
  ], {
    x: 11.0, y: 7.15, w: 1.5, h: 0.25, fontSize: 9,
    align: "right", charSpacing: 2, fontFace: FONT, margin: 0,
  });
}

function circle(slide, x, y, size, color, text = "", textColor = C.bg) {
  slide.addShape(pres.shapes.OVAL, {
    x, y, w: size, h: size,
    fill: { color }, line: { color, width: 0 },
  });
  if (text) {
    slide.addText(text, {
      x, y, w: size, h: size, fontSize: size * 28,
      color: textColor, bold: true,
      align: "center", valign: "middle",
      fontFace: FONT, margin: 0,
    });
  }
}

// ============================================================
// SLIDE 1 · 封面
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  // 背景光晕
  s.addShape(pres.shapes.OVAL, {
    x: 9.5, y: -2.5, w: 7, h: 7,
    fill: { color: C.gold, transparency: 85 }, line: { color: C.gold, width: 0 },
  });
  s.addShape(pres.shapes.OVAL, {
    x: -2.5, y: 5, w: 6, h: 6,
    fill: { color: C.cyan, transparency: 92 }, line: { color: C.cyan, width: 0 },
  });

  // Eyebrow
  s.addText("PROJECT PLAN · 2026 HACKATHON", {
    x: 0.9, y: 1.0, w: 9, h: 0.35,
    fontSize: 12, fontFace: FONT, color: C.gold,
    bold: true, charSpacing: 8, margin: 0,
  });

  // 主标题
  s.addText("文化官", {
    x: 0.9, y: 1.7, w: 11, h: 1.4,
    fontSize: T.coverTitle, fontFace: FONT, color: C.white,
    bold: true, margin: 0,
  });

  // 副标题 — gold
  s.addText("AI 智能运营平台", {
    x: 0.9, y: 3.15, w: 11, h: 0.9,
    fontSize: T.coverSub, fontFace: FONT, color: C.gold,
    bold: true, margin: 0,
  });

  // 一句话 tagline
  s.addText("钉钉里的企业文化 AI 运营官 · 开放生态平台", {
    x: 0.9, y: 4.15, w: 11, h: 0.4,
    fontSize: T.coverTag, fontFace: FONT, color: C.light, margin: 0,
  });

  // 三个 pills
  const pills = [
    { t: "AI 智能化", c: C.gold },
    { t: "开放 MCP 生态", c: C.cyan },
    { t: "SaaS 想象空间", c: C.pink },
  ];
  pills.forEach((p, i) => {
    const x = 0.9 + i * 2.3;
    s.addShape(pres.shapes.ROUNDED_RECTANGLE, {
      x, y: 5.0, w: 2.1, h: 0.45,
      fill: { color: C.bgCard }, line: { color: p.c, width: 1 },
      rectRadius: 0.22,
    });
    s.addText(p.t, {
      x, y: 5.0, w: 2.1, h: 0.45,
      fontSize: 11, fontFace: FONT, color: p.c,
      bold: true, align: "center", valign: "middle", margin: 0,
    });
  });

  // Bottom meta
  s.addText("Go (Gin) · MCP Server · 4 周交付 · 单二进制部署", {
    x: 0.9, y: 6.3, w: 10, h: 0.3,
    fontSize: 10, fontFace: FONT, color: C.muted, charSpacing: 3, margin: 0,
  });

  footer(s, 1);
}

// ============================================================
// SLIDE 2 · 一句话定位（金句）
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  eyebrow(s, "POSITIONING · 项目定位");

  // 大引号装饰
  s.addText("“", {
    x: 0.7, y: 1.0, w: 1.2, h: 1.5,
    fontSize: 130, fontFace: "Georgia", color: C.gold, margin: 0, bold: true,
  });

  // 金句（大字）
  s.addText("把企业文化变成", {
    x: 1.9, y: 1.55, w: 11, h: 0.95,
    fontSize: 40, fontFace: FONT, color: C.white,
    bold: true, margin: 0,
  });
  s.addText([
    { text: "可被任意 AI 操控", options: { color: C.gold, bold: true } },
    { text: "的中台", options: { color: C.white, bold: true } },
  ], {
    x: 1.9, y: 2.55, w: 11, h: 0.95, fontSize: 40, fontFace: FONT, margin: 0,
  });

  // 副注
  s.addText("—— 我们不是把 Excel 搬上线，而是重构企业文化运营的底层范式", {
    x: 1.9, y: 3.7, w: 11, h: 0.4,
    fontSize: 13, fontFace: FONT, color: C.muted, italic: true, margin: 0,
  });

  // 三段式：燃料 / 大脑 / 神经
  const triples = [
    { k: "积分", v: "燃料", c: C.gold, sub: "可量化的文化激励单位" },
    { k: "AI", v: "大脑", c: C.cyan, sub: "Agent 自动编排全链路运营" },
    { k: "MCP", v: "神经接口", c: C.pink, sub: "开放给任意 AI 客户端调用" },
  ];
  triples.forEach((t, i) => {
    const x = 0.9 + i * 4.1;
    const y = 5.0;
    card(s, { x, y, w: 3.8, h: 1.6, fill: C.bgCard, border: C.border });
    // small accent bar (left side only, allowed since not full-width)
    s.addShape(pres.shapes.RECTANGLE, {
      x, y, w: 0.08, h: 1.6,
      fill: { color: t.c }, line: { color: t.c, width: 0 },
    });
    s.addText([
      { text: t.k, options: { color: t.c, bold: true, fontSize: 22 } },
      { text: "  =  ", options: { color: C.muted, fontSize: 14 } },
      { text: t.v, options: { color: C.white, bold: true, fontSize: 22 } },
    ], {
      x: x + 0.3, y: y + 0.25, w: 3.5, h: 0.6, fontFace: FONT, margin: 0, valign: "middle",
    });
    s.addText(t.sub, {
      x: x + 0.3, y: y + 0.9, w: 3.4, h: 0.55,
      fontSize: 11, fontFace: FONT, color: C.muted, margin: 0,
    });
  });

  footer(s, 2);
}

// ============================================================
// SLIDE 3 · 痛点（2x2 大数字）
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  eyebrow(s, "CHAPTER 01 · 背景与痛点");
  pageTitle(s, "Excel 时代的文化官");
  pageSub(s, "误差不可追溯 · 流程不透明 · 规则不闭环 · 沉淀难量化");

  const pains = [
    { n: "-4", label: "图一 · 误扣分案例", note: "Excel 无审计日志，陈先生被错扣 4 分至今无法追溯。", c: C.red },
    { n: "100%", label: "全人工 HR 加减分", note: "手工操作公平性存疑，员工申诉无据可依。", c: C.gold },
    { n: "0", label: "扫码即扣分 BUG", note: "缺少 TCC 事务校验 — 没抽奖也被扣分（图二）。", c: C.pink },
    { n: "∞", label: "无成就沉淀", note: "频次低、无积累，文化氛围难以量化。", c: C.cyan },
  ];

  pains.forEach((p, i) => {
    const col = i % 2, row = Math.floor(i / 2);
    const x = 0.7 + col * 6.1;
    const y = 2.2 + row * 2.45;
    card(s, { x, y, w: 5.9, h: 2.25 });
    // 大数字
    s.addText(p.n, {
      x: x + 0.4, y: y + 0.2, w: 2.5, h: 1.1,
      fontSize: T.bigNum, fontFace: FONT, color: p.c,
      bold: true, margin: 0, valign: "top",
    });
    // 标签
    s.addText(p.label, {
      x: x + 0.4, y: y + 1.3, w: 5.2, h: 0.35,
      fontSize: 13, fontFace: FONT, color: C.white,
      bold: true, margin: 0,
    });
    // 说明
    s.addText(p.note, {
      x: x + 0.4, y: y + 1.65, w: 5.2, h: 0.5,
      fontSize: 11, fontFace: FONT, color: C.muted, margin: 0,
    });
  });

  footer(s, 3);
}

// ============================================================
// SLIDE 4 · 三大叙事（3 列）
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  eyebrow(s, "CHAPTER 01 · 项目定位");
  pageTitle(s, "三大差异化叙事");
  pageSub(s, "AI 智能化 · 开放生态 · SaaS 想象 — 路演反复强调的三条主线");

  const narr = [
    {
      no: "01", k: "AI 智能化", c: C.gold,
      headline: "HR-Agent · 自然语言运营",
      body: "HR 一句话即可发起活动 — 等于一支文化运营团队。建活动、发卡片、群机器人、生成海报，全自动跑完。",
      tech: "Claude 4.6 Function Calling + Go MCP 工具集",
    },
    {
      no: "02", k: "开放生态", c: C.cyan,
      headline: "MCP Protocol · 跨客户端",
      body: "把平台能力 MCP 化 — Claude Desktop / ChatGPT / Cursor 等任意 AI 客户端均可调用。国内首个企业内场景 MCP 落地。",
      tech: "自研 JSON-RPC over SSE",
    },
    {
      no: "03", k: "SaaS 想象", c: C.pink,
      headline: "跨企业文化指数榜",
      body: "脱敏 + opt-in 的跨企业指数榜 — 从一个企业工具，进化为整个行业的文化数据基础设施。",
      tech: "Phase 1 演示 → Phase 2 邀请 → Phase 3 基础设施",
    },
  ];

  narr.forEach((n, i) => {
    const x = 0.7 + i * 4.13;
    const y = 2.15;
    const w = 3.93;
    const h = 4.6;
    card(s, { x, y, w, h });
    // 顶部色带（左侧细条）
    s.addShape(pres.shapes.RECTANGLE, {
      x, y, w: 0.1, h: h,
      fill: { color: n.c }, line: { color: n.c, width: 0 },
    });
    // 编号
    s.addText(n.no, {
      x: x + 0.35, y: y + 0.3, w: 1.5, h: 0.6,
      fontSize: 36, fontFace: FONT, color: n.c,
      bold: true, margin: 0,
    });
    // 类别 chip
    s.addText(n.k, {
      x: x + 0.35, y: y + 1.0, w: 3.4, h: 0.35,
      fontSize: 11, fontFace: FONT, color: n.c, bold: true,
      charSpacing: 4, margin: 0,
    });
    // 标题
    s.addText(n.headline, {
      x: x + 0.35, y: y + 1.45, w: 3.4, h: 0.7,
      fontSize: 18, fontFace: FONT, color: C.white,
      bold: true, margin: 0,
    });
    // 正文
    s.addText(n.body, {
      x: x + 0.35, y: y + 2.25, w: 3.4, h: 1.6,
      fontSize: 12, fontFace: FONT, color: C.light, margin: 0,
    });
    // 分隔线
    s.addShape(pres.shapes.LINE, {
      x: x + 0.35, y: y + 3.85, w: 3.3, h: 0,
      line: { color: C.border, width: 0.75 },
    });
    // 技术注
    s.addText(n.tech, {
      x: x + 0.35, y: y + 3.95, w: 3.4, h: 0.55,
      fontSize: 10, fontFace: FONT, color: C.muted, italic: true, margin: 0,
    });
  });

  footer(s, 4);
}

// ============================================================
// SLIDE 5 · 王牌① HR-Agent
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  eyebrow(s, "HIGHLIGHT ① · 王牌功能");
  pageTitle(s, "HR-Agent · 自然语言一键运营");
  pageSub(s, "HR 一句话 → Agent 自动跑完 6 步全链路编排");

  // 左：对话引用
  const lx = 0.7, ly = 2.15, lw = 6.0, lh = 4.55;
  card(s, { x: lx, y: ly, w: lw, h: lh });
  // 顶部 chip
  s.addShape(pres.shapes.ROUNDED_RECTANGLE, {
    x: lx + 0.3, y: ly + 0.3, w: 1.5, h: 0.32,
    fill: { color: C.goldSoft }, line: { color: C.gold, width: 0.5 },
    rectRadius: 0.16,
  });
  s.addText("HR 后台 · 输入", {
    x: lx + 0.3, y: ly + 0.3, w: 1.5, h: 0.32,
    fontSize: 9, fontFace: FONT, color: C.gold, bold: true,
    align: "center", valign: "middle", charSpacing: 1, margin: 0,
  });
  // 引用
  s.addText("“给销售部发一场月底冲刺活动，下周三 18:00，奖励 50 分，限 30 人，海报用红色科技风”", {
    x: lx + 0.3, y: ly + 0.8, w: lw - 0.6, h: 1.5,
    fontSize: 17, fontFace: FONT, color: C.white,
    italic: true, bold: true, margin: 0,
  });
  // 分隔
  s.addShape(pres.shapes.LINE, {
    x: lx + 0.3, y: ly + 2.4, w: lw - 0.6, h: 0,
    line: { color: C.border, width: 0.75 },
  });
  // 终端式回复（标题 + 报告 + 汇总合并到一个文本块，避免溢出错位）
  s.addText([
    { text: "AGENT 执行报告", options: { color: C.cyan, bold: true, fontSize: 10, charSpacing: 3, breakLine: true } },
    { text: " ", options: { fontSize: 6, breakLine: true } },
    { text: "✓  活动已创建（ID: ACT-20260530）", options: { color: C.green, fontSize: 11, breakLine: true } },
    { text: "✓  钉钉日程已发送 · 销售部 28 人", options: { color: C.green, fontSize: 11, breakLine: true } },
    { text: "✓  红色科技风海报已生成", options: { color: C.green, fontSize: 11, breakLine: true } },
    { text: "✓  群机器人已播报", options: { color: C.green, fontSize: 11, breakLine: true } },
    { text: "✓  互动卡片已推送", options: { color: C.green, fontSize: 11, breakLine: true } },
    { text: " ", options: { fontSize: 6, breakLine: true } },
    { text: "→  耗时 8.4s · 5 个工具 · 全部成功", options: { color: C.gold, bold: true, fontSize: 11 } },
  ], {
    x: lx + 0.3, y: ly + 2.55, w: lw - 0.6, h: 1.95,
    fontFace: "Consolas", margin: 0, paraSpaceAfter: 2, valign: "top",
  });

  // 右：6 步纵向
  const rx = 7.0, ry = 2.15, rw = 5.6, rh = 4.55;
  card(s, { x: rx, y: ry, w: rw, h: rh });
  s.addText("Agent 自动执行 · 6 步编排", {
    x: rx + 0.3, y: ry + 0.25, w: rw - 0.6, h: 0.35,
    fontSize: 13, fontFace: FONT, color: C.white, bold: true, margin: 0,
  });

  const steps = [
    { n: "1", t: "活动创建", d: "写入 DB · 设置规则参数" },
    { n: "2", t: "钉钉日程", d: "给目标员工建日程邀请" },
    { n: "3", t: "内容生成", d: "海报 + 群推送文案" },
    { n: "4", t: "互动卡片", d: "群内发可点击报名卡" },
    { n: "5", t: "群机器人", d: "部门群播报" },
    { n: "6", t: "执行报告", d: "返回耗时 / 成功率 / 工单" },
  ];
  steps.forEach((step, i) => {
    const sy = ry + 0.75 + i * 0.62;
    // 圆形编号
    s.addShape(pres.shapes.OVAL, {
      x: rx + 0.3, y: sy, w: 0.42, h: 0.42,
      fill: { color: C.gold }, line: { color: C.gold, width: 0 },
    });
    s.addText(step.n, {
      x: rx + 0.3, y: sy, w: 0.42, h: 0.42,
      fontSize: 14, fontFace: FONT, color: C.bg, bold: true,
      align: "center", valign: "middle", margin: 0,
    });
    // 标题
    s.addText(step.t, {
      x: rx + 0.85, y: sy - 0.02, w: 2.0, h: 0.3,
      fontSize: 13, fontFace: FONT, color: C.white, bold: true, margin: 0,
    });
    // 描述
    s.addText(step.d, {
      x: rx + 0.85, y: sy + 0.24, w: 4.5, h: 0.25,
      fontSize: 10, fontFace: FONT, color: C.muted, margin: 0,
    });
  });

  footer(s, 5);
}

// ============================================================
// SLIDE 6 · 王牌② 开放 MCP
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  eyebrow(s, "HIGHLIGHT ② · 技术深度爆点");
  pageTitle(s, "开放 API + MCP Server");
  pageSub(s, "把平台能力全部 MCP 化 · 任意 AI 客户端可调用");

  // 上：两层开放
  const tw = 6.0, th = 2.0, ty = 2.15;
  // REST
  card(s, { x: 0.7, y: ty, w: tw, h: th, fill: C.bgCardSoft, border: C.border });
  s.addText("REST OpenAPI", {
    x: 1.0, y: ty + 0.25, w: 4.0, h: 0.4,
    fontSize: 16, fontFace: FONT, color: C.white, bold: true, margin: 0,
  });
  s.addText("标准 API + OAuth 接入", {
    x: 1.0, y: ty + 0.7, w: 5.2, h: 0.35,
    fontSize: 12, fontFace: FONT, color: C.light, margin: 0,
  });
  s.addText("适合：企业自有系统对接、传统集成场景", {
    x: 1.0, y: ty + 1.1, w: 5.2, h: 0.6,
    fontSize: 10, fontFace: FONT, color: C.muted, margin: 0,
  });

  // MCP（金色高亮）
  card(s, { x: 7.0, y: ty, w: tw - 0.4, h: th, fill: C.goldSoft, border: C.gold });
  s.addShape(pres.shapes.ROUNDED_RECTANGLE, {
    x: 7.3, y: ty + 0.25, w: 0.8, h: 0.32,
    fill: { color: C.gold }, line: { color: C.gold, width: 0 },
    rectRadius: 0.16,
  });
  s.addText("★ 王牌", {
    x: 7.3, y: ty + 0.25, w: 0.8, h: 0.32,
    fontSize: 9, fontFace: FONT, color: C.bg, bold: true,
    align: "center", valign: "middle", margin: 0,
  });
  s.addText("MCP Server", {
    x: 8.25, y: ty + 0.25, w: 3.5, h: 0.4,
    fontSize: 16, fontFace: FONT, color: C.gold, bold: true, margin: 0,
  });
  s.addText("完整工具集：查积分 / 加分 / 发活动 / 查排名 / 颁发徽章", {
    x: 7.3, y: ty + 0.7, w: tw - 0.7, h: 0.35,
    fontSize: 12, fontFace: FONT, color: C.white, margin: 0,
  });
  s.addText("适合：Claude Desktop · ChatGPT · Cursor · 企业自有 Agent", {
    x: 7.3, y: ty + 1.1, w: tw - 0.7, h: 0.6,
    fontSize: 10, fontFace: FONT, color: C.light, margin: 0,
  });

  // 下：Demo 杀手锏剧本
  const dy = 4.4, dh = 2.35;
  card(s, { x: 0.7, y: dy, w: 11.9, h: dh, fill: C.bgCard });
  s.addText("Demo 杀手锏 · 现场剧本", {
    x: 1.0, y: dy + 0.2, w: 6, h: 0.4,
    fontSize: 14, fontFace: FONT, color: C.white, bold: true, margin: 0,
  });
  s.addShape(pres.shapes.ROUNDED_RECTANGLE, {
    x: 10.6, y: dy + 0.22, w: 1.7, h: 0.32,
    fill: { color: C.pinkSoft }, line: { color: C.pink, width: 0.75 },
    rectRadius: 0.16,
  });
  s.addText("CRITICAL · 75s", {
    x: 10.6, y: dy + 0.22, w: 1.7, h: 0.32,
    fontSize: 9, fontFace: FONT, color: C.pink, bold: true,
    align: "center", valign: "middle", charSpacing: 1, margin: 0,
  });

  const demo = [
    { n: "1", t: "打开 Claude Desktop", d: "使用已知稳定版本 · 预录视频备份" },
    { n: "2", t: "连接 MCP Server", d: "SSE 长连接 · 工具列表自动加载" },
    { n: "3", t: "输入指令", d: "“查本月销售部前 3 名，给第一加 100 分，颁发『销售之星』”" },
    { n: "4", t: "Claude 直接执行 ✓", d: "评委亲眼见证 · 跨客户端能力被调用" },
  ];
  demo.forEach((d, i) => {
    const dx = 0.95 + i * 2.95;
    const dyy = dy + 0.85;
    // 圆数字
    s.addShape(pres.shapes.OVAL, {
      x: dx, y: dyy, w: 0.5, h: 0.5,
      fill: { color: i === 3 ? C.gold : C.bgCardSoft },
      line: { color: i === 3 ? C.gold : C.border, width: 0.75 },
    });
    s.addText(d.n, {
      x: dx, y: dyy, w: 0.5, h: 0.5,
      fontSize: 16, fontFace: FONT, color: i === 3 ? C.bg : C.gold,
      bold: true, align: "center", valign: "middle", margin: 0,
    });
    s.addText(d.t, {
      x: dx, y: dyy + 0.55, w: 2.7, h: 0.35,
      fontSize: 12, fontFace: FONT, color: i === 3 ? C.gold : C.white,
      bold: true, margin: 0,
    });
    s.addText(d.d, {
      x: dx, y: dyy + 0.9, w: 2.7, h: 0.85,
      fontSize: 10, fontFace: FONT, color: C.muted, margin: 0,
    });
  });

  footer(s, 6);
}

// ============================================================
// SLIDE 7 · 5 项辅助亮点
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  eyebrow(s, "HIGHLIGHT ③ - ⑦ · 辅助亮点");
  pageTitle(s, "5 项 AI 亮点 · 张力拉满");
  pageSub(s, "内容生成 · 数据洞察 · 盲盒主持 · 跨企业榜 · 反作弊");

  const items = [
    { no: "③", t: "AI 内容生成助手", c: C.cyan,
      d: "一句话生成活动文案 / 海报 / 钉钉推送话术 / 品牌二维码 · HR 确认后全渠道分发",
      tag: "Claude / 通义万相 / SDXL" },
    { no: "④", t: "AI 文化数据洞察 Dashboard", c: C.gold,
      d: "「为什么研发部参与度下降 30%？」→ SQL Agent 归因 + RAG 历史 + 策略推荐 — 给老板看的杀手锏",
      tag: "SQL Agent + RAG" },
    { no: "⑤", t: "盲盒 AI 互动主持人", c: C.pink,
      d: "抽奖弹出 Live2D 形象 + TTS 语音 + LLM 实时文案 — 现场张力拉满，评委记忆点之一",
      tag: "Live2D + Edge-TTS + LLM" },
    { no: "⑥", t: "跨企业文化指数榜", c: C.purple,
      d: "脱敏 + opt-in · 企业文化指数与行业排行（互联网/电商/外贸）—— SaaS 想象空间入口",
      tag: "脱敏聚合 · 多租户" },
    { no: "⑦", t: "AI 反作弊与异常检测", c: C.red,
      d: "同 IP 多扫码 / 积分异常膨胀 / 兑换频次异常 → 孤立森林 + LLM 解释 → 自动工单",
      tag: "孤立森林 + LLM" },
  ];

  items.forEach((it, i) => {
    const y = 2.15 + i * 0.95;
    card(s, { x: 0.7, y, w: 11.9, h: 0.85, fill: C.bgCard });
    // 左侧细色条
    s.addShape(pres.shapes.RECTANGLE, {
      x: 0.7, y, w: 0.08, h: 0.85,
      fill: { color: it.c }, line: { color: it.c, width: 0 },
    });
    // 编号大字
    s.addText(it.no, {
      x: 0.95, y: y + 0.1, w: 0.7, h: 0.65,
      fontSize: 28, fontFace: FONT, color: it.c, bold: true,
      align: "center", valign: "middle", margin: 0,
    });
    // 标题
    s.addText(it.t, {
      x: 1.75, y: y + 0.08, w: 6.5, h: 0.35,
      fontSize: 14, fontFace: FONT, color: C.white, bold: true, margin: 0,
    });
    // 描述
    s.addText(it.d, {
      x: 1.75, y: y + 0.42, w: 8.3, h: 0.42,
      fontSize: 11, fontFace: FONT, color: C.light, margin: 0,
    });
    // 技术 tag
    s.addShape(pres.shapes.ROUNDED_RECTANGLE, {
      x: 10.3, y: y + 0.28, w: 2.2, h: 0.32,
      fill: { color: C.bg },
      line: { color: it.c, width: 0.5 },
      rectRadius: 0.16,
    });
    s.addText(it.tag, {
      x: 10.3, y: y + 0.28, w: 2.2, h: 0.32,
      fontSize: 9, fontFace: FONT, color: it.c, bold: true,
      align: "center", valign: "middle", margin: 0,
    });
  });

  footer(s, 7);
}

// ============================================================
// SLIDE 8 · 钉钉边界（三层）
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  eyebrow(s, "CHAPTER 03 · 钉钉深度集成");
  pageTitle(s, "钉钉集成边界 · 已核验官方 API");
  pageSub(s, "透明告知：✓ 实施 6 项 / △ 谨慎使用 / × 自建签到");

  // 三栏
  const cols = [
    {
      title: "✓  实施清单", sub: "6 项官方 API",
      c: C.green, fill: "0E2A28",
      items: [
        ["活动日程", "接受 = 报名"],
        ["互动卡片", "群内可点击报名"],
        ["工作通知", "asyncsend_v2"],
        ["群机器人", "周报 / 月报 / 名单"],
        ["OA 审批", "异议 / 大额调整"],
        ["工作台卡片", "首页一屏直达"],
      ]
    },
    {
      title: "△  谨慎使用", sub: "配额受限",
      c: C.gold, fill: "2D2614",
      items: [
        ["DING 强提醒", "仅企业自建 + 专业版"],
        ["", "只用于关键事件"],
        ["", "中奖通知 / 异议结果"],
        ["", ""],
        ["配额预算", "钉钉专业版"],
        ["50万次/月", "标准版几天爆配额"],
      ]
    },
    {
      title: "×  自建模块", sub: "钉钉 API 不支持",
      c: C.pink, fill: "2D1A28",
      items: [
        ["签到模块自研", "考勤 API 只读"],
        ["", ""],
        ["H5 + 二维码", "动态刷新"],
        ["GPS / WiFi 围栏", "精度校验"],
        ["防代签", "随机题校验"],
        ["", ""],
      ]
    },
  ];

  cols.forEach((col, i) => {
    const x = 0.7 + i * 4.13;
    const y = 2.15;
    const w = 3.93;
    const h = 4.6;
    card(s, { x, y, w, h, fill: col.fill, border: col.c });
    // 标题
    s.addText(col.title, {
      x: x + 0.3, y: y + 0.3, w: w - 0.6, h: 0.5,
      fontSize: 22, fontFace: FONT, color: col.c, bold: true, margin: 0,
    });
    s.addText(col.sub, {
      x: x + 0.3, y: y + 0.85, w: w - 0.6, h: 0.3,
      fontSize: 10, fontFace: FONT, color: C.muted,
      charSpacing: 3, margin: 0,
    });
    // 分隔线
    s.addShape(pres.shapes.LINE, {
      x: x + 0.3, y: y + 1.25, w: w - 0.6, h: 0,
      line: { color: col.c, width: 0.5 },
    });
    // 条目
    col.items.forEach((row, ri) => {
      const ry = y + 1.45 + ri * 0.5;
      if (row[0]) {
        s.addText(row[0], {
          x: x + 0.3, y: ry, w: w - 0.6, h: 0.25,
          fontSize: 12, fontFace: FONT, color: C.white, bold: true, margin: 0,
        });
      }
      if (row[1]) {
        s.addText(row[1], {
          x: x + 0.3, y: ry + 0.22, w: w - 0.6, h: 0.25,
          fontSize: 10, fontFace: FONT, color: C.muted, margin: 0,
        });
      }
    });
  });

  footer(s, 8);
}

// ============================================================
// SLIDE 9 · 4 周排期（横向甘特）
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  eyebrow(s, "CHAPTER 04 · 里程碑与排期");
  pageTitle(s, "4 周交付 · 黑客马拉松排期");
  pageSub(s, "2026-05-22 起 · 必保 5 项 · 可砍 2 项");

  const weeks = [
    { w: "W1", date: "05/22 - 05/28", c: C.gold, items: [
      { t: "项目骨架", d: "Go + 多租户", dur: "2d" },
      { t: "核心 CRUD", d: "积分 / 活动 / 商品", dur: "3d" },
      { t: "★ 自研签到", d: "H5+QR+GPS", dur: "4d" },
    ]},
    { w: "W2", date: "05/29 - 06/04", c: C.cyan, items: [
      { t: "★ HR-Agent", d: "编排器 + MCP 工具", dur: "4d" },
      { t: "★ 对外 MCP", d: "JSON-RPC + REST", dur: "3d" },
    ]},
    { w: "W3", date: "06/05 - 06/11", c: C.pink, items: [
      { t: "★ 钉钉集成", d: "日程 / 卡片 / 机器人", dur: "4d" },
      { t: "数据洞察", d: "SQL Agent + RAG", dur: "3d" },
      { t: "跨企业榜", d: "脱敏 + 虚拟对手", dur: "3d" },
    ]},
    { w: "W4", date: "06/12 - 06/18", c: C.purple, items: [
      { t: "工作台卡片", d: "一屏直达", dur: "2d" },
      { t: "★ 盲盒主持", d: "TTS + Live2D", dur: "3d" },
      { t: "Demo 彩排", d: "录制 + 现场", dur: "2d" },
    ]},
  ];

  // 左 9.2: 四行；右 3.1: 必保/可砍
  weeks.forEach((wk, i) => {
    const y = 2.15 + i * 1.18;
    const rowH = 1.05;
    // 周标签
    s.addShape(pres.shapes.RECTANGLE, {
      x: 0.7, y, w: 1.2, h: rowH,
      fill: { color: wk.c }, line: { color: wk.c, width: 0 },
    });
    s.addText(wk.w, {
      x: 0.7, y: y + 0.1, w: 1.2, h: 0.5,
      fontSize: 22, fontFace: FONT, color: C.bg, bold: true,
      align: "center", margin: 0,
    });
    s.addText(wk.date, {
      x: 0.7, y: y + 0.6, w: 1.2, h: 0.35,
      fontSize: 9, fontFace: FONT, color: C.bg,
      align: "center", margin: 0,
    });
    // 任务卡片
    wk.items.forEach((it, ii) => {
      const ix = 2.0 + ii * 2.4;
      card(s, { x: ix, y, w: 2.3, h: rowH, fill: C.bgCard, shadow: false });
      // duration chip
      s.addShape(pres.shapes.ROUNDED_RECTANGLE, {
        x: ix + 1.7, y: y + 0.13, w: 0.5, h: 0.28,
        fill: { color: C.bg },
        line: { color: wk.c, width: 0.5 },
        rectRadius: 0.14,
      });
      s.addText(it.dur, {
        x: ix + 1.7, y: y + 0.13, w: 0.5, h: 0.28,
        fontSize: 9, fontFace: FONT, color: wk.c, bold: true,
        align: "center", valign: "middle", margin: 0,
      });
      s.addText(it.t, {
        x: ix + 0.2, y: y + 0.12, w: 1.5, h: 0.35,
        fontSize: 12, fontFace: FONT, color: C.white, bold: true, margin: 0,
      });
      s.addText(it.d, {
        x: ix + 0.2, y: y + 0.5, w: 2.0, h: 0.5,
        fontSize: 10, fontFace: FONT, color: C.muted, margin: 0,
      });
    });
  });

  // 右下：必保 / 可砍
  card(s, { x: 9.5, y: 2.15, w: 3.1, h: 4.55, fill: C.bgCardSoft });
  s.addText("交付优先级", {
    x: 9.7, y: 2.3, w: 2.8, h: 0.35,
    fontSize: 12, fontFace: FONT, color: C.white, bold: true, margin: 0,
  });
  // 必保
  s.addShape(pres.shapes.RECTANGLE, {
    x: 9.7, y: 2.8, w: 0.08, h: 1.85,
    fill: { color: C.gold }, line: { color: C.gold, width: 0 },
  });
  s.addText("✓ 必保 (MUST)", {
    x: 9.85, y: 2.78, w: 2.8, h: 0.3,
    fontSize: 11, fontFace: FONT, color: C.gold, bold: true, margin: 0,
  });
  ["① HR-Agent", "② 开放 MCP", "⑤ 盲盒主持", "★ 自研签到", "★ 钉钉日程/卡片"].forEach((t, i) => {
    s.addText(t, {
      x: 9.85, y: 3.1 + i * 0.32, w: 2.8, h: 0.28,
      fontSize: 11, fontFace: FONT, color: C.light, margin: 0,
    });
  });
  // 可砍
  s.addShape(pres.shapes.RECTANGLE, {
    x: 9.7, y: 4.95, w: 0.08, h: 1.05,
    fill: { color: C.red }, line: { color: C.red, width: 0 },
  });
  s.addText("△ 时间不够可砍", {
    x: 9.85, y: 4.93, w: 2.8, h: 0.3,
    fontSize: 11, fontFace: FONT, color: C.red, bold: true, margin: 0,
  });
  ["⑦ 反作弊（首砍）", "⑥ 跨企业（留 1 虚拟对手）"].forEach((t, i) => {
    s.addText(t, {
      x: 9.85, y: 5.27 + i * 0.3, w: 2.8, h: 0.28,
      fontSize: 11, fontFace: FONT, color: C.light, margin: 0,
    });
  });
  // 总周期
  s.addText("28 天 · 单二进制部署", {
    x: 9.7, y: 6.3, w: 2.8, h: 0.3,
    fontSize: 10, fontFace: FONT, color: C.muted, charSpacing: 2, margin: 0,
  });

  footer(s, 9);
}

// ============================================================
// SLIDE 10 · Demo 脚本 + 时间分配
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  eyebrow(s, "CHAPTER 05 · 路演 Demo");
  pageTitle(s, "5 分钟 Demo 脚本");
  pageSub(s, "7 帧 · MCP 杀手锏在第 3 帧 · 备份预录视频兜底");

  const frames = [
    { n: 1, dur: "30s", t: "开场 · 痛点", d: "Excel 痛点截图（图一图二）引出问题" },
    { n: 2, dur: "60s", t: "HR-Agent", d: "HR 说“发个销售部冲刺活动” · 评委手机现场收到钉钉推送" },
    { n: 3, dur: "75s", t: "★ 开放 MCP", d: "Claude Desktop 连 MCP Server · 一句话查前 3 并加分 · Claude 直接执行 ✓", hero: true },
    { n: 4, dur: "45s", t: "签到 + 加分", d: "员工扫码 → 自研签到校验 → 积分入账 + 成就徽章" },
    { n: 5, dur: "60s", t: "盲盒抽奖", d: "AI 主持 Live2D + 语音互动 · 中奖瞬间张力拉满" },
    { n: 6, dur: "60s", t: "洞察 + 跨企业榜", d: "“为什么参与度下降” → AI 归因 + 我司互联网行业第 3" },
    { n: 7, dur: "15s", t: "收官", d: "架构图 + 三大叙事金句" },
  ];

  // 左：脚本列表
  frames.forEach((f, i) => {
    const y = 2.15 + i * 0.62;
    card(s, { x: 0.7, y, w: 8.0, h: 0.55,
      fill: f.hero ? C.goldSoft : C.bgCard, border: f.hero ? C.gold : C.border,
      shadow: false });
    s.addShape(pres.shapes.OVAL, {
      x: 0.85, y: y + 0.1, w: 0.35, h: 0.35,
      fill: { color: f.hero ? C.gold : C.bgCardSoft },
      line: { color: f.hero ? C.gold : C.border, width: 0.5 },
    });
    s.addText(String(f.n), {
      x: 0.85, y: y + 0.1, w: 0.35, h: 0.35,
      fontSize: 12, fontFace: FONT, color: f.hero ? C.bg : C.gold, bold: true,
      align: "center", valign: "middle", margin: 0,
    });
    s.addText(f.dur, {
      x: 1.3, y: y + 0.15, w: 0.7, h: 0.3,
      fontSize: 11, fontFace: "Consolas", color: f.hero ? C.gold : C.muted, bold: true, margin: 0,
    });
    s.addText(f.t, {
      x: 2.05, y: y + 0.05, w: 2.0, h: 0.45,
      fontSize: 13, fontFace: FONT,
      color: f.hero ? C.gold : C.white, bold: true, valign: "middle", margin: 0,
    });
    s.addText(f.d, {
      x: 4.1, y: y + 0.05, w: 4.5, h: 0.45,
      fontSize: 11, fontFace: FONT, color: C.light, valign: "middle", margin: 0,
    });
  });

  // 右：时间分配可视化 + 总览
  const rx = 9.0, ry = 2.15, rw = 3.6;
  card(s, { x: rx, y: ry, w: rw, h: 2.5, fill: C.bgCardSoft });
  s.addText("时间分配", {
    x: rx + 0.2, y: ry + 0.2, w: rw - 0.4, h: 0.35,
    fontSize: 12, fontFace: FONT, color: C.white, bold: true, margin: 0,
  });
  const totals = [
    { n: "痛点", v: 30 }, { n: "Agent", v: 60 }, { n: "★ MCP", v: 75, hero: true },
    { n: "签到", v: 45 }, { n: "盲盒", v: 60 }, { n: "洞察", v: 60 }, { n: "收官", v: 15 },
  ];
  const maxV = Math.max(...totals.map(x => x.v));
  totals.forEach((t, i) => {
    const yy = ry + 0.65 + i * 0.26;
    s.addText(t.n, {
      x: rx + 0.2, y: yy, w: 0.85, h: 0.22,
      fontSize: 10, fontFace: FONT, color: t.hero ? C.gold : C.muted,
      bold: t.hero, margin: 0,
    });
    // bar 容器
    s.addShape(pres.shapes.RECTANGLE, {
      x: rx + 1.15, y: yy + 0.06, w: 1.8, h: 0.12,
      fill: { color: C.border }, line: { color: C.border, width: 0 },
    });
    // bar fill
    s.addShape(pres.shapes.RECTANGLE, {
      x: rx + 1.15, y: yy + 0.06, w: 1.8 * (t.v / maxV), h: 0.12,
      fill: { color: t.hero ? C.gold : C.cyan },
      line: { color: t.hero ? C.gold : C.cyan, width: 0 },
    });
    s.addText(t.v + "s", {
      x: rx + 3.0, y: yy, w: 0.5, h: 0.22,
      fontSize: 9, fontFace: "Consolas", color: t.hero ? C.gold : C.muted,
      bold: true, margin: 0,
    });
  });

  // 总览数据
  card(s, { x: rx, y: 4.85, w: rw, h: 1.85, fill: C.goldSoft, border: C.gold });
  s.addText("5'45\"", {
    x: rx + 0.2, y: 4.95, w: rw - 0.4, h: 0.7,
    fontSize: 38, fontFace: FONT, color: C.gold, bold: true, margin: 0,
  });
  s.addText("总时长", {
    x: rx + 0.2, y: 5.65, w: rw - 0.4, h: 0.25,
    fontSize: 10, fontFace: FONT, color: C.muted, charSpacing: 3, margin: 0,
  });
  s.addShape(pres.shapes.LINE, {
    x: rx + 0.2, y: 6.0, w: rw - 0.4, h: 0,
    line: { color: C.border, width: 0.5 },
  });
  s.addText("关键张力点：第 3 帧 MCP 调用", {
    x: rx + 0.2, y: 6.1, w: rw - 0.4, h: 0.3,
    fontSize: 11, fontFace: FONT, color: C.white, bold: true, margin: 0,
  });
  s.addText("备份方案：预录视频兜底", {
    x: rx + 0.2, y: 6.4, w: rw - 0.4, h: 0.25,
    fontSize: 10, fontFace: FONT, color: C.light, margin: 0,
  });

  footer(s, 10);
}

// ============================================================
// SLIDE 11 · 评委一页纸（8 维度）
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  eyebrow(s, "CHAPTER 06 · 竞赛亮点");
  pageTitle(s, "评委一页纸 · 8 维度评分亮点");
  pageSub(s, "创新 · AI · 深度 · 生态 · 商业 · 严谨 · 演示 · 落地 — 每维都有杀手锏");

  const dims = [
    { t: "创新性", h: "国内首个 MCP Agent 协议", d: "在企业内部场景落地", c: C.gold },
    { t: "AI 浓度", h: "5 大 AI 模块", d: "Agent / 内容 / 洞察 / 反作弊 / 主持", c: C.gold },
    { t: "技术深度", h: "MCP 自研 + 多 Agent", d: "Go JSON-RPC + 多租户 + TCC 事务", c: C.cyan },
    { t: "生态价值", h: "任意 AI 客户端可调", d: "Claude / ChatGPT / Cursor", c: C.cyan },
    { t: "商业想象", h: "跨企业 SaaS 雏形", d: "中国企业文化数据平台", c: C.pink },
    { t: "工程严谨", h: "钉钉 API 边界已核验", d: "专业版预算 · 签到自建", c: C.green },
    { t: "可演示", h: "Demo 张力拉满", d: "Claude Desktop + AI 语音主持", c: C.purple },
    { t: "可落地", h: "4 周可交付", d: "已对接钉钉 · 单二进制部署", c: C.gold },
  ];

  dims.forEach((d, i) => {
    const col = i % 4, row = Math.floor(i / 4);
    const x = 0.7 + col * 3.05;
    const y = 2.15 + row * 2.3;
    const w = 2.85, h = 2.1;
    card(s, { x, y, w, h });
    // 左侧细色条
    s.addShape(pres.shapes.RECTANGLE, {
      x, y, w: 0.08, h,
      fill: { color: d.c }, line: { color: d.c, width: 0 },
    });
    // 维度名
    s.addText(d.t, {
      x: x + 0.28, y: y + 0.25, w: w - 0.4, h: 0.3,
      fontSize: 11, fontFace: FONT, color: d.c, bold: true,
      charSpacing: 3, margin: 0,
    });
    // 亮点 headline
    s.addText(d.h, {
      x: x + 0.28, y: y + 0.65, w: w - 0.4, h: 0.85,
      fontSize: 15, fontFace: FONT, color: C.white, bold: true, margin: 0,
    });
    // 描述
    s.addText(d.d, {
      x: x + 0.28, y: y + 1.5, w: w - 0.4, h: 0.5,
      fontSize: 11, fontFace: FONT, color: C.muted, margin: 0,
    });
  });

  footer(s, 11);
}

// ============================================================
// SLIDE 12 · 三大金句收尾
// ============================================================
{
  const s = pres.addSlide();
  bg(s);

  // 背景装饰
  s.addShape(pres.shapes.OVAL, {
    x: 9.5, y: -3, w: 7, h: 7,
    fill: { color: C.gold, transparency: 88 }, line: { color: C.gold, width: 0 },
  });
  s.addShape(pres.shapes.OVAL, {
    x: -2.5, y: 4, w: 6, h: 6,
    fill: { color: C.pink, transparency: 92 }, line: { color: C.pink, width: 0 },
  });

  eyebrow(s, "CLOSING · 三大叙事金句");
  s.addText("路演反复强调的", {
    x: 0.7, y: 0.85, w: 11, h: 0.6,
    fontSize: 22, fontFace: FONT, color: C.muted, margin: 0,
  });
  s.addText("三句话", {
    x: 0.7, y: 1.4, w: 11, h: 0.8,
    fontSize: 40, fontFace: FONT, color: C.gold, bold: true, margin: 0,
  });

  // 三个引用，渐次缩进展示
  const quotes = [
    { t: "我们不是把 Excel 搬上线，而是把企业文化变成可被任意 AI 操控的中台。", c: C.gold },
    { t: "HR 一句话，钉钉里全套自动跑完。", c: C.cyan },
    { t: "今天是一个企业的工具，明天是整个行业的基础设施。", c: C.pink },
  ];

  quotes.forEach((q, i) => {
    const y = 2.6 + i * 0.95;
    // 左侧色条
    s.addShape(pres.shapes.RECTANGLE, {
      x: 0.7, y, w: 0.1, h: 0.78,
      fill: { color: q.c }, line: { color: q.c, width: 0 },
    });
    // 编号
    s.addText(`0${i+1}`, {
      x: 0.95, y: y + 0.05, w: 0.7, h: 0.65,
      fontSize: 26, fontFace: FONT, color: q.c, bold: true, margin: 0,
    });
    // 引用文本
    s.addText("“" + q.t + "”", {
      x: 1.75, y: y, w: 10.7, h: 0.78,
      fontSize: 18, fontFace: FONT, color: C.white,
      bold: true, italic: true, valign: "middle", margin: 0,
    });
  });

  // 收尾
  s.addText("THANK YOU · Q & A", {
    x: 0.7, y: 6.0, w: 11.9, h: 0.5,
    fontSize: 22, fontFace: FONT, color: C.gold, bold: true,
    align: "center", charSpacing: 8, margin: 0,
  });
  s.addText("CULTURE POINTS MALL · AI OPERATING PLATFORM · v4", {
    x: 0.7, y: 6.55, w: 11.9, h: 0.3,
    fontSize: 9, fontFace: FONT, color: C.muted,
    align: "center", charSpacing: 4, margin: 0,
  });

  footer(s, 12);
}

// 写出文件
pres.writeFile({ fileName: "../文化官方案-v4.pptx" }).then(fn => {
  console.log("OK: " + fn);
}).catch(e => {
  console.error("ERR:", e);
  process.exit(1);
});
