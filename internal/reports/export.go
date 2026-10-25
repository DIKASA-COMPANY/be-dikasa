package reports

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Export calculates a fresh workbook for all matching rows, ignoring page slicing.
func (s *Service) Export(ctx context.Context, actor string, k Kind, f Filter) ([]byte, error) {
	r, e := s.read(ctx, actor, k, f, false)
	if e != nil {
		return nil, e
	}
	return workbook(r)
}

// workbook shares the exact report calculation and writes untrusted labels as inline strings.
func workbook(r Result) ([]byte, error) {
	rows := [][]any{}
	switch r.Data.Kind {
	case Summary:
		rows = append(rows, []any{"Tanggal WIB", "Transaksi", "Nilai Dibayar", "Refund", "Penjualan Bersih"})
		for _, d := range r.Data.Daily {
			rows = append(rows, []any{d.Date, d.SaleCount, d.ChargedSales, d.RefundAmount, d.NetSales})
		}
	case Methods:
		rows = append(rows, []any{"Metode", "Transaksi", "Penjualan Dibayar", "Jumlah Refund", "Refund", "Setelah Refund"})
		for _, m := range r.Data.Methods {
			rows = append(rows, []any{m.Method, m.SaleCount, m.SalesAmount, m.RefundCount, m.RefundAmount, m.NetAmount})
		}
	default:
		rows = append(rows, []any{"Nama", "Terjual", "Direfund", "Unit Bersih", "Penjualan Kotor", "Diskon", "Setelah Diskon", "Pajak", "Refund Net", "Refund Pajak", "Refund", "Penjualan Bersih", "Pajak Bersih", "Setelah Refund", "Kategori"})
		for _, g := range r.Data.Rows {
			rows = append(rows, []any{g.Name, g.SoldQuantity, g.ReturnedQuantity, g.NetQuantity, g.GrossSales, g.DiscountAmount, g.SalesAfterDiscount, g.SaleTax, g.RefundNet, g.RefundTax, g.RefundAmount, g.NetSales, g.NetTax, g.ValueAfterRefunds, g.CategoryName})
		}
	}
	rows = append(rows, []any{}, []any{"Laporan", string(r.Data.Kind)}, []any{"Mulai", r.Meta.StartDate}, []any{"Sampai", r.Meta.EndDate}, []any{"Zona Waktu", r.Meta.Timezone}, []any{"Pencarian", r.Meta.Query}, []any{"Urutan", r.Meta.Sort}, []any{"Dibuat", r.Meta.GeneratedAt.Format(time.RFC3339)}, []any{"Penjualan Kotor", r.Data.Totals.GrossSales}, []any{"Refund", r.Data.Totals.RefundAmount}, []any{"Penjualan Bersih (tanpa pajak)", r.Data.Totals.NetSales}, []any{"Pajak Bersih", r.Data.Totals.NetTax}, []any{"Nilai Setelah Refund", r.Data.Totals.ValueAfterRefunds}, []any{"Riwayat Dikecualikan", r.Data.Totals.ExcludedReceipts}, []any{"Kategori historis tidak tercatat (unit)", r.Data.Totals.UnknownCategoryQuantity}, []any{"Aturan", "Penjualan pada tanggal bayar; refund pada tanggal pengembalian. Penjualan bersih tanpa pajak. Nilai dapat negatif. Kategori lama tidak ditebak."})
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="15" width="24" customWidth="1"/></cols><sheetData>`)
	for r, row := range rows {
		fmt.Fprintf(&sheet, `<row r="%d">`, r+1)
		for c, value := range row {
			if ptr, ok := value.(*int64); ok {
				if ptr == nil {
					value = nil
				} else {
					value = *ptr
				}
			}
			reference := string(rune('A'+c)) + strconv.Itoa(r+1)
			style := 0
			if r == 0 {
				style = 1
			}
			switch v := value.(type) {
			case nil:
				fmt.Fprintf(&sheet, `<c r="%s"/>`, reference)
			case int64:
				fmt.Fprintf(&sheet, `<c r="%s" s="2"><v>%d</v></c>`, reference, v)
			default:
				var escaped bytes.Buffer
				_ = xml.EscapeText(&escaped, []byte(fmt.Sprint(v)))
				fmt.Fprintf(&sheet, `<c r="%s" s="%d" t="inlineStr"><is><t xml:space="preserve">%s</t></is></c>`, reference, style, escaped.String())
			}
		}
		sheet.WriteString(`</row>`)
	}
	fmt.Fprintf(&sheet, `</sheetData><autoFilter ref="A1:O%d"/></worksheet>`, len(rows))
	parts := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Laporan" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`,
		"xl/styles.xml":              `<?xml version="1.0"?><styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="2"><font><sz val="11"/><name val="Calibri"/></font><font><b/><sz val="11"/><name val="Calibri"/></font></fonts><fills count="2"><fill><patternFill patternType="none"/></fill><fill><patternFill patternType="gray125"/></fill></fills><borders count="1"><border/></borders><cellStyleXfs count="1"><xf numFmtId="0" fontId="0" fillId="0" borderId="0"/></cellStyleXfs><cellXfs count="3"><xf numFmtId="0" fontId="0" fillId="0" borderId="0" xfId="0"/><xf numFmtId="0" fontId="1" fillId="0" borderId="0" xfId="0"/><xf numFmtId="3" fontId="0" fillId="0" borderId="0" xfId="0" applyNumberFormat="1"/></cellXfs></styleSheet>`,
		"xl/worksheets/sheet1.xml":   sheet.String(),
	}
	var buffer bytes.Buffer
	writer := zip.NewWriter(&buffer)
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/styles.xml", "xl/worksheets/sheet1.xml"} {
		entry, err := writer.Create(name)
		if err != nil {
			return nil, err
		}
		if _, err = entry.Write([]byte(parts[name])); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
