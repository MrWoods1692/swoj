# 比赛成绩表导出

比赛详情页的排行榜支持一键导出为 6 种格式，方便老师/管理员分发、归档与统计。

## 端点

`GET /api/contests/{id}/rank/export?format=<format>`

- 权限：公开（与 `/api/contests/{id}/rank` 一致）
- 参数：`format` 取值 `csv | tsv | json | markdown | html | xlsx`，缺省为 `csv`
- 返回：`Content-Disposition: attachment`，附带 `X-Contest-ID` / `X-Contest-Name` / `X-Format` / `X-Rows` 响应头便于脚本断言

## 排名口径

与 `contestRank` 完全一致：`submissions` 表 `contest_id=? AND status=?`（`status` 为 `StatusAccepted`），按 `COUNT(DISTINCT problem_id) DESC, MIN(created_at) ASC` 排序，最多 100 行。

- 每行为一名选手的排名快照：名次、用户名、去重通过题数、首次通过时间
- 同一题目多次提交只算一次去重通过
- 首次通过时间字段来自 `MIN(created_at)`，SQLite 存的是 `CURRENT_TIMESTAMP` 字符串（`YYYY-MM-DD HH:MM:SS`），因此 handler 直接 Scan 成 `string`，modernc.org/sqlite 不会隐式解析成 `time.Time`

## 支持的格式

| 格式 | Content-Type | 特点 | 使用场景 |
|---|---|---|---|
| `csv` | `text/csv` | UTF-8 BOM + 逗号分隔，Excel/Numbers/WPS 双击即开 | 通用，老师分发 |
| `tsv` | `text/tab-separated-values` | UTF-8 BOM + 制表符分隔 | 数据管道、shell `column` |
| `json` | `application/json` | 数组，字段 `rank/username/accepted/first_at` | 程序消费 |
| `markdown` | `text/markdown` | 管道表格 + H1 + 人数统计 | 粘贴到 issue/README/Wiki |
| `html` | `text/html` | 完整 HTML + 内联样式 + CSP `default-src 'none'` | 浏览器直接预览 |
| `xlsx` | `spreadsheetml.sheet` | OOXML SpreadsheetML，纯 stdlib `archive/zip` + `encoding/xml` | Excel/WPS 原生打开 |

## 表头字段

固定四列，顺序与 CSV/XLSX/HTML 一致：

1. 名次（从 1 开始，按 rank 端点的排序结果）
2. 用户名
3. 通过题数（去重后）
4. 首次通过时间（`YYYY-MM-DD HH:MM:SS`）

## xlsx 手写实现说明

xlsx 不引入第三方库（如 `excelize`），用标准库构造最小 OOXML 包：

- `[Content_Types].xml`：声明包内 part 的 MIME
- `_rels/.rels`：包根关系，指向 workbook
- `xl/workbook.xml`：sheet 名（默认取比赛名，去掉 `<>&"'`）
- `xl/_rels/workbook.xml.rels`：workbook → worksheet
- `xl/worksheets/sheet1.xml`：实际数据

单元格策略：
- 表头一律 `t="inlineStr"` 内联字符串
- 纯数字（含空字符串）走 `<v>` 数值单元格
- 其他字符串走 `t="inlineStr"` + `<is><t>…</t></is>`

列字母用 `colLetter(n)` 换算：`1→A`、`26→Z`、`27→AA`。

## 前端

`web/src/views/ContestDetail.vue` 排行榜面板右上角显示「导出」按钮组：CSV / TSV / Excel / JSON / Markdown / HTML。调用 `api.download()`：

- `api.download(path, fallbackName)` 是 `api.js` 新增的下载封装：`fetch` + `Blob` + `<a download>` 触发，从 `Content-Disposition` 解析文件名，回退到 fallback
- 只在比赛开放排行榜（`contest.rank === true`）时显示按钮组
- 下载成功弹出 toast「导出成功」，失败用中文错误提示

## 验证

`verify_export.py` 覆盖：

- 匿名访问、非法 format、不存在的 contest、非数字 id → 400 / 404
- 6 种格式的 Content-Type / Content-Disposition / X-* 响应头
- csv/tsv 的 UTF-8 BOM + 表头 + 数据行 + 分隔符
- json 数组结构与字段名
- markdown 的 H1 + 分隔行 + 数据行
- html 的 `<!doctype html>` + `<table>` + CSP 头
- xlsx 的 zip 结构（4 个必需 part）+ sheet 内含用户名
- 排名口径：3 用户 8 提交（含同题多次提交）→ Alice=3 / Carol=2 / Bob=2
- 空结果：只有表头一行

## 相关文件

- `internal/app/handler_export.go` — 6 种格式输出
- `internal/app/handler_contest.go:226` — `contestRank`，导出复用其查询口径
- `internal/app/router.go` — 路由注册
- `web/src/api.js` — `api.download`
- `web/src/views/ContestDetail.vue` — 前端按钮组
- `verify_export.py` — 72 项断言
