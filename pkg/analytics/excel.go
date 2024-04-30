package analytics

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/xuri/excelize/v2"
)

func GenerateTradesExcel(trades []Trade) (*os.File, error) {
	xlsx := excelize.NewFile()
	sheetName := "Trades"
	index, err := xlsx.NewSheet(sheetName)
	if err != nil {
		return nil, err
	}
	xlsx.SetSheetName(xlsx.GetSheetName(1), sheetName)
	xlsx.SetActiveSheet(index)
	xlsx.DeleteSheet("Sheet1")

	// Set titles for the columns
	titles := []string{"Account ID", "Symbol ID", "Quantity", "Entry Price", "Exit Price", "Entered At", "Exited At", "Profit or Loss"}
	for i, title := range titles {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1) // Columns start at 1, not 0
		xlsx.SetCellValue(sheetName, cell, title)
	}

	// Inserting data
	for i, trade := range trades {
		row := i + 2 // Starting from the second row, since the first row is the header
		xlsx.SetCellValue(sheetName, fmt.Sprintf("A%d", row), trade.AccountID)
		xlsx.SetCellValue(sheetName, fmt.Sprintf("B%d", row), trade.SymbolID)
		xlsx.SetCellValue(sheetName, fmt.Sprintf("C%d", row), trade.Quantity)
		xlsx.SetCellValue(sheetName, fmt.Sprintf("D%d", row), trade.EntryPrice)
		xlsx.SetCellValue(sheetName, fmt.Sprintf("E%d", row), trade.ExitPrice)
		xlsx.SetCellValue(sheetName, fmt.Sprintf("F%d", row), trade.EnteredAt)
		xlsx.SetCellValue(sheetName, fmt.Sprintf("G%d", row), trade.ExitedAt)
		xlsx.SetCellValue(sheetName, fmt.Sprintf("H%d", row), trade.ProfitOrLoss)
	}

	// Create temporary file
	tempFile, err := ioutil.TempFile(os.TempDir(), "trades-*.xlsx")
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()

	// Save to the temporary file
	if err := xlsx.SaveAs(tempFile.Name()); err != nil {
		return nil, err
	}

	return tempFile, nil
}

func CleanupTempDir(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Minute)
	for {
		select {
		case <-ticker.C:
			files, err := ioutil.ReadDir(os.TempDir())
			if err != nil {
				return
			}
			for _, file := range files {
				if time.Since(file.ModTime()) > 10*time.Minute {
					os.Remove(os.TempDir() + "/" + file.Name())
				}
			}
		case <-ctx.Done():
			ticker.Stop()
			return
		}
	}
}
