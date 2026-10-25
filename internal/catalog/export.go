package catalog

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// ExportWorkbook exports all matching outlet products and variants as literal-text XLSX rows for managers.
func (s *Service) ExportWorkbook(ctx context.Context, userID string, filter ProductFilter, productID, availability string) ([]byte, error) {
	outlet, err := s.scope(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	filter.OutletID = outlet
	filter.PageSize = 100
	var products []Product
	for page := 1; ; page++ {
		filter.Page = page
		rows, total, e := s.repository.ListProducts(ctx, filter)
		if e != nil {
			return nil, e
		}
		products = append(products, rows...)
		if len(rows) == 0 || int64(len(products)) >= total {
			break
		}
	}
	var selected []Product
	for _, p := range products {
		if (productID == "" || p.ID == productID) && (availability == "" || p.EffectiveStockStatus() == availability) {
			selected = append(selected, p)
		}
	}
	if productID != "" && len(selected) == 0 {
		return nil, ErrNotFound
	}
	return productWorkbook(selected)
}

// productWorkbook uses literal inline strings and counts shared inventory only on base-product rows.
func productWorkbook(products []Product) ([]byte, error) {
	rows := [][]any{{"ID Produk", "Nama Produk", "Kategori", "Jenis Produk", "Variasi", "Harga Jual", "Harga Online", "Harga Modal", "Modal Online", "Status Manual", "Ketersediaan", "Lacak Stok", "Jumlah Stok", "Batas Menipis", "Aktif", "Catatan"}}
	for _, p := range products {
		var quantity, threshold any
		if p.TrackStock {
			quantity = p.StockQuantity
			threshold = p.LowStockThreshold
		}
		rows = append(rows, []any{p.ID, p.Name, p.Category.Name, p.Category.ProductType.Code, "", p.Price, p.OnlineSellingPrice, p.CostPrice, p.OnlineCostPrice, p.StockStatus, p.EffectiveStockStatus(), p.TrackStock, quantity, threshold, p.IsActive, p.Description})
		for _, v := range p.Variants {
			if !v.IsActive {
				continue
			}
			rows = append(rows, []any{p.ID, p.Name, p.Category.Name, p.Category.ProductType.Code, v.Name, v.Price, v.OnlineSellingPrice, v.CostPrice, v.OnlineCostPrice, v.StockStatus, "", "", nil, nil, v.IsActive, ""})
		}
	}
	var sheet strings.Builder
	sheet.WriteString(`<?xml version="1.0" encoding="UTF-8"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetViews><sheetView workbookViewId="0"><pane ySplit="1" topLeftCell="A2" state="frozen"/></sheetView></sheetViews><cols><col min="1" max="16" width="24" customWidth="1"/></cols><sheetData>`)
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
	fmt.Fprintf(&sheet, `</sheetData><autoFilter ref="A1:P%d"/></worksheet>`, len(rows))
	parts := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.styles+xml"/></Types>`,
		"_rels/.rels":                `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Produk" sheetId="1" r:id="rId1"/></sheets></workbook>`,
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
