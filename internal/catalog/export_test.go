package catalog

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

type workbookRepo struct {
	fakeRepository
	pages int
}

func (r *workbookRepo) ListProducts(_ context.Context, f ProductFilter) ([]Product, int64, error) {
	r.pages++
	return []Product{{ID: "p", Name: "=SUM(1,2) Ελληνικά", Price: 123, StockStatus: "available", Category: ProductCategory{Name: "Food"}}}, 2, nil
}
func TestWorkbookIsRealXLSXWithLiteralStringsAndAllPages(t *testing.T) {
	repo := &workbookRepo{}
	s := NewService(repo, fakeAccountFinder{account: account("manager")}, nil)
	data, err := s.ExportWorkbook(context.Background(), "u", ProductFilter{}, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if repo.pages != 2 {
		t.Fatal(repo.pages)
	}
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, file := range reader.File {
		if file.Name == "xl/worksheets/sheet1.xml" {
			found = true
			r, _ := file.Open()
			b, _ := io.ReadAll(r)
			r.Close()
			text := string(b)
			if !strings.Contains(text, `t="inlineStr"`) || !strings.Contains(text, "=SUM(1,2) Ελληνικά") || strings.Contains(text, "<f>") || !strings.Contains(text, "<v>123</v>") {
				t.Fatal(text)
			}
		}
	}
	if !found {
		t.Fatal("missing sheet")
	}
	s = NewService(repo, fakeAccountFinder{account: account("cashier")}, nil)
	if _, err = s.ExportWorkbook(context.Background(), "u", ProductFilter{}, "", ""); !errors.Is(err, ErrForbidden) {
		t.Fatal(err)
	}
}
