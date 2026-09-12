package app

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// 比赛成绩表导出：把 /api/contests/{id}/rank 的结果按指定格式输出。
//
// 支持格式：csv / tsv / json / markdown / html / xlsx。
// - csv / tsv：UTF-8 BOM 保证 Excel 打开不乱码，分隔符 \t 或 ,
// - json：直接序列化 rank 行数组
// - markdown：管道表格，便于粘贴到 issue/README
// - html：Excel 兼容的 <table> 片段，浏览器直接可预览
// - xlsx：用 archive/zip + encoding/xml 手写最小 xlsx，不引入第三方依赖
//
// 权限：与 contestRank 一致，公开。前端 ContestDetail.vue 会按 rank 开关决定是否显示按钮。

// rankExportFormat 校验白名单。
var rankExportFormats = map[string]struct{}{
	"csv":      {},
	"tsv":      {},
	"json":     {},
	"markdown": {},
	"html":     {},
	"xlsx":     {},
}

// rankRawEntry 从 submissions 聚合出的排行榜行（内部结构，不序列化）。
type rankRawEntry struct {
	Username string
	Accepted int
	FirstAt  string
}

// rankEntryJSON JSON 响应体：字段名与 contestRank 一致。
type rankEntryJSON struct {
	Username string `json:"username"`
	Accepted int    `json:"accepted"`
	FirstAt  string `json:"first_at"`
	Rank     int    `json:"rank"`
}

// rankHeader 表头字段（多格式共用）。
var rankHeader = []string{"名次", "用户名", "通过题数", "首次通过时间"}

// sepRow 返回与 headers 等长的 "---" 分隔行，用于 Markdown 表格。
func sepRow(headers []string) []string {
	row := make([]string, len(headers))
	for i := range row {
		row[i] = "---"
	}
	return row
}

// rankRowsToSlice 把内部结构转成 [][]string（表头 + 数据行），供 csv/tsv/markdown/html/xlsx 共用。
func rankRowsToSlice(list []rankRawEntry, startRank int) [][]string {
	rows := make([][]string, 0, len(list)+1)
	rows = append(rows, rankHeader)
	for i, e := range list {
		rows = append(rows, []string{
			strconv.Itoa(startRank + i),
			e.Username,
			strconv.Itoa(e.Accepted),
			e.FirstAt,
		})
	}
	return rows
}

// rankRowsToJSON 把内部结构转成 JSON 行（带名次）。
func rankRowsToJSON(list []rankRawEntry, startRank int) []rankEntryJSON {
	out := make([]rankEntryJSON, 0, len(list))
	for i, e := range list {
		out = append(out, rankEntryJSON{
			Username: e.Username,
			Accepted: e.Accepted,
			FirstAt:  e.FirstAt,
			Rank:     startRank + i,
		})
	}
	return out
}

// contestRankExport 输出比赛排行榜（GET /api/contests/{id}/rank/export?format=csv）。
func (s *Server) contestRankExport(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		Fail(w, http.StatusBadRequest, "比赛编号无效")
		return
	}
	format := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("format")))
	if format == "" {
		format = "csv"
	}
	if _, ok := rankExportFormats[format]; !ok {
		Fail(w, http.StatusBadRequest, "不支持的导出格式")
		return
	}
	// 读取比赛：不存在返回 404
	var name, start, end string
	err := s.db.QueryRow(`SELECT name, start_time, end_time FROM contests WHERE id=?`, id).
		Scan(&name, &start, &end)
	if err != nil {
		Fail(w, http.StatusNotFound, "比赛不存在")
		return
	}
	// 复用 contestRank 的查询口径：contest_id + status=AC，按去重题数降序、首次通过时间升序
	// created_at 存的是 SQLite CURRENT_TIMESTAMP 格式（"YYYY-MM-DD HH:MM:SS"，TEXT），
	// modernc.org/sqlite 不解析成 time.Time，直接 Scan 成 string。
	rows, err := s.db.Query(`SELECT username, COUNT(DISTINCT problem_id),
		COALESCE(MIN(created_at), '') AS first_at
		FROM submissions WHERE contest_id=? AND status=?
		GROUP BY username
		ORDER BY COUNT(DISTINCT problem_id) DESC, MIN(created_at) ASC LIMIT 100`,
		id, StatusAccepted)
	if err != nil {
		Fail(w, http.StatusInternalServerError, err.Error())
		return
	}
	defer rows.Close()
	list := []rankRawEntry{}
	for rows.Next() {
		var e rankRawEntry
		if err := rows.Scan(&e.Username, &e.Accepted, &e.FirstAt); err == nil {
			list = append(list, e)
		}
	}
	// 生成 Content-Disposition 文件名（含比赛 ID 与时间戳，避免浏览器缓存同名）
	ts := time.Now().Format("20060102-150405")
	baseName := fmt.Sprintf("contest-%d-rank-%s", id, ts)
	mime, ext := rankExportMeta(format)
	w.Header().Set("Content-Type", mime)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.%s"`, baseName, ext))
	w.Header().Set("X-Contest-ID", strconv.FormatInt(id, 10))
	w.Header().Set("X-Contest-Name", name)
	w.Header().Set("X-Format", format)
	w.Header().Set("X-Rows", strconv.Itoa(len(list)))

	switch format {
	case "csv":
		writeRankCSV(w, list, ",")
	case "tsv":
		writeRankCSV(w, list, "\t")
	case "json":
		writeRankJSON(w, list)
	case "markdown":
		writeRankMarkdown(w, name, list)
	case "html":
		writeRankHTML(w, name, list)
	case "xlsx":
		if err := writeRankXLSX(w, name, list); err != nil {
			Fail(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
}

// rankExportMeta 返回每种格式的 Content-Type 与文件扩展名。
func rankExportMeta(format string) (string, string) {
	switch format {
	case "csv":
		return "text/csv; charset=utf-8", "csv"
	case "tsv":
		return "text/tab-separated-values; charset=utf-8", "tsv"
	case "json":
		return "application/json; charset=utf-8", "json"
	case "markdown":
		return "text/markdown; charset=utf-8", "md"
	case "html":
		return "text/html; charset=utf-8", "html"
	case "xlsx":
		return "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"
	}
	return "application/octet-stream", "bin"
}

// writeRankCSV 写 csv/tsv：BOM 头确保 Excel 识别 UTF-8；行数据来自 rankRowsToSlice。
func writeRankCSV(w http.ResponseWriter, list []rankRawEntry, sep string) {
	// UTF-8 BOM
	if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil {
		return
	}
	c := csv.NewWriter(w)
	switch sep {
	case "\t":
		c.Comma = '\t'
	default:
		c.Comma = ','
	}
	rows := rankRowsToSlice(list, 1)
	_ = c.WriteAll(rows)
	c.Flush()
	if err := c.Error(); err != nil {
		return
	}
}

// writeRankJSON 写 JSON 数组。
func writeRankJSON(w http.ResponseWriter, list []rankRawEntry) {
	out := rankRowsToJSON(list, 1)
	_ = json.NewEncoder(w).Encode(out)
}

// writeRankMarkdown 写 Markdown 管道表格。
func writeRankMarkdown(w http.ResponseWriter, name string, list []rankRawEntry) {
	fmt.Fprintf(w, "# %s · 比赛成绩表\n\n", escapeMD(name))
	fmt.Fprintf(w, "_共 %d 人参与_\n\n", len(list))
	fmt.Fprintf(w, "| %s |\n", strings.Join(rankHeader, " | "))
	fmt.Fprintf(w, "| %s |\n", strings.Join(sepRow(rankHeader), " | "))
	rows := rankRowsToSlice(list, 1)
	for i := 1; i < len(rows); i++ {
		fmt.Fprintf(w, "| %s |\n", strings.Join(rows[i], " | "))
	}
}

// writeRankHTML 写浏览器可预览的 HTML 表格。
func writeRankHTML(w http.ResponseWriter, name string, list []rankRawEntry) {
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	tn := htmlEscape(name)
	ts := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(w, `<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8"><title>%s · 比赛成绩表</title>
<style>
body{font:14px/1.6 -apple-system,system-ui,sans-serif;padding:24px;color:#111}
h1{font-size:20px;margin-bottom:4px}
.meta{color:#666;margin-bottom:12px}
table{border-collapse:collapse;width:100%%}
th,td{border:1px solid #ddd;padding:6px 10px;text-align:left}
th{background:#f5f5f5}
tr:nth-child(even) td{background:#fafafa}
</style></head><body>
<h1>%s · 比赛成绩表</h1>
<div class="meta">共 %d 人参与 · 导出时间 %s</div>
<table><thead><tr>`, tn, tn, len(list), ts)
	for _, h := range rankHeader {
		fmt.Fprintf(w, "<th>%s</th>", htmlEscape(h))
	}
	fmt.Fprintf(w, "</tr></thead><tbody>")
	rows := rankRowsToSlice(list, 1)
	for i := 1; i < len(rows); i++ {
		fmt.Fprintf(w, "<tr>")
		for _, cell := range rows[i] {
			fmt.Fprintf(w, "<td>%s</td>", htmlEscape(cell))
		}
		fmt.Fprintf(w, "</tr>")
	}
	fmt.Fprintf(w, "</tbody></table></body></html>")
}

// escapeMD 转义 Markdown 表格里的管道与换行。
func escapeMD(s string) string {
	s = strings.ReplaceAll(s, "|", `\-`)
	s = strings.ReplaceAll(s, "\n", "<br>")
	s = strings.ReplaceAll(s, "\r", "")
	return s
}

// htmlEscape 最小 HTML 转义。
func htmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "\"", "&quot;", "'", "&#39;")
	return r.Replace(s)
}

// writeRankXLSX 手写最小 xlsx：zip + XML，只含单 sheet，字符串走 inlineStr。
//
// XLSX 结构（OOXML SpreadsheetML）：
//   [Content_Types].xml         声明包内所有 part 的 MIME
//   _rels/.rels                 包根关系：指向 workbook
//   xl/workbook.xml             工作簿：声明 sheet 名称
//   xl/_rels/workbook.xml.rels  workbook 与 sheet 的关系
//   xl/worksheets/sheet1.xml    实际数据：行列 + 单元格值
//
// 不写 sharedStrings：全部字符串用 inlineStr 直接写在单元格，简单可靠。
func writeRankXLSX(w http.ResponseWriter, name string, list []rankRawEntry) error {
	zw := zip.NewWriter(w)
	defer zw.Close()

	writeZipEntry := func(name string, data string) error {
		f, err := zw.Create(name)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, data)
		return err
	}

	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
<Default Extension="xml" ContentType="application/xml"/>
<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>
<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>
</Types>`
	if err := writeZipEntry("[Content_Types].xml", contentTypes); err != nil {
		return err
	}

	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>
</Relationships>`
	if err := writeZipEntry("_rels/.rels", rels); err != nil {
		return err
	}

	sheetName := strings.TrimSpace(name)
	if sheetName == "" {
		sheetName = "Sheet1"
	}
	sheetNameEsc := htmlEscape(sheetName)
	workbook := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
<sheets><sheet name="` + sheetNameEsc + `" sheetId="1" r:id="rId1"/></sheets>
</workbook>`
	if err := writeZipEntry("xl/workbook.xml", workbook); err != nil {
		return err
	}

	workbookRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>
</Relationships>`
	if err := writeZipEntry("xl/_rels/workbook.xml.rels", workbookRels); err != nil {
		return err
	}

	// 组 sheet XML
	rows := rankRowsToSlice(list, 1)
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`)
	b.WriteString(`<sheetData>`)
	for ri, row := range rows {
		rn := ri + 1
		b.WriteString(fmt.Sprintf(`<row r="%d">`, rn))
		for ci, cell := range row {
			col := colLetter(ci + 1)
			ref := col + strconv.Itoa(rn)
			if ri == 0 {
				// 表头：inlineStr 且加粗（加粗需要 styles.xml；这里保持 plain，不引入额外 part）
				b.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, htmlEscape(cell)))
			} else if isAllDigits(cell) {
				b.WriteString(fmt.Sprintf(`<c r="%s"><v>%s</v></c>`, ref, cell))
			} else {
				b.WriteString(fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, htmlEscape(cell)))
			}
		}
		b.WriteString(`</row>`)
	}
	b.WriteString(`</sheetData></worksheet>`)
	return writeZipEntry("xl/worksheets/sheet1.xml", b.String())
}

// colLetter 列号转字母：1→A, 26→Z, 27→AA。
func colLetter(n int) string {
	if n < 1 {
		n = 1
	}
	s := ""
	for n > 0 {
		n--
		s = string(rune('A'+n%26)) + s
		n /= 26
	}
	return s
}

// isAllDigits 用于判断单元格能否当数字写；空字符串也视为数字（写 0）。
func isAllDigits(s string) bool {
	if s == "" {
		return true
	}
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}
