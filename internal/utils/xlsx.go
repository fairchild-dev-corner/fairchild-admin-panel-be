package utils

import (
	"bytes"
	"time"

	"github.com/xuri/excelize/v2"
)

// BuildXLSX writes one sheet: a dark, frozen header row, then rows. Strings
// are stored as text, so member IDs and mobile numbers keep their leading
// zeros and nothing a member typed can run as a formula. widths are column
// widths in characters, one per header.
func BuildXLSX(sheet string, headers []string, widths []float64, rows [][]any) ([]byte, error) {
	f := excelize.NewFile()
	defer f.Close()
	if err := f.SetSheetName(f.GetSheetName(0), sheet); err != nil {
		return nil, err
	}

	titles := make([]any, len(headers))
	for i, h := range headers {
		titles[i] = h
	}
	if err := f.SetSheetRow(sheet, "A1", &titles); err != nil {
		return nil, err
	}
	last, err := excelize.CoordinatesToCellName(len(headers), 1)
	if err != nil {
		return nil, err
	}
	style, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"173A5E"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return nil, err
	}
	if err := f.SetCellStyle(sheet, "A1", last, style); err != nil {
		return nil, err
	}

	for i, row := range rows {
		cell, err := excelize.CoordinatesToCellName(1, i+2)
		if err != nil {
			return nil, err
		}
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			return nil, err
		}
	}

	for i, w := range widths {
		col, err := excelize.ColumnNumberToName(i + 1)
		if err != nil {
			return nil, err
		}
		if err := f.SetColWidth(sheet, col, col, w); err != nil {
			return nil, err
		}
	}
	if err := f.SetPanes(sheet, &excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ManilaDateTime turns an RFC 3339 time into "2026-11-19 14:05" in
// Asia/Manila, or "" when it's empty or unreadable.
func ManilaDateTime(rfc3339 string) string {
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return ""
	}
	return t.In(ManilaTZ).Format("2006-01-02 15:04")
}
