package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/tradingcage/tradingcage-go/pkg/bars"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Gorm model
type Model struct {
	ID        uint `gorm:"primaryKey"`
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt gorm.DeletedAt `gorm:"index"`
}

// TagsInstances represents a record in the tags_instances table
type TagsInstances struct {
	gorm.Model
	Datetime time.Time `gorm:"not null;index:idx_tags_instances,unique"`
	SymbolID uint      `gorm:"not null;index:idx_tags_instances,unique"`
	TagID    uint      `gorm:"not null;index:idx_tags_instances,unique"`
	UserID   uint      `gorm:"not null;index:idx_tags_instances,unique"`
}

// Tags represents a record in the tags table
type Tags struct {
	gorm.Model
	Name   string `gorm:"not null;size:255"`
	UserID uint   `gorm:"not null;index"`
}

// TagsCollections represents a record in the tags_collections table
type TagsCollections struct {
	gorm.Model
	TagID        uint `gorm:"not null;index"`
	CollectionID uint `gorm:"not null;index"`
}

// Collections represents a record in the collections table
type Collections struct {
	gorm.Model
	Name   string `gorm:"not null;size:255"`
	UserID uint   `gorm:"not null;index"`
}

// TagMetaData provides additional data and actions for a tag
type TagMetaData struct {
	ID           uint
	GetInstances func(db *gorm.DB) ([]TagsInstances, error)
}

// TagNames holds the global variable for tag names mapping
var TagNames = map[string]TagMetaData{
	"All Day Bull Trend": {
		GetInstances: getAllDayBullTrendTagsInstances,
	},
	"All Day Bear Trend": {
		GetInstances: getAllDayBearTrendTagsInstances,
	},
}

const batchSize = 1000

func main() {
	// Initialize your database connection; adjust as needed for your environment.
	dsn := os.Getenv("TIMESCALE_URL")
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}

	initializeDatabase(db)

	for tag, metadata := range TagNames {
		if metadata.GetInstances == nil {
			log.Println("Skipping tag", tag)
			continue
		}
		instances, err := metadata.GetInstances(db)
		if err != nil {
			log.Fatalf("error getting instances for tag %s: %v\n", tag, err)
		}
		if len(instances) > 0 {
			// Set TagID for all instances
			for index := range instances {
				instances[index].TagID = metadata.ID
			}
			// Bulk insert with ON CONFLICT DO NOTHING to handle duplicates
			for i := 0; i < len(instances); i += batchSize {
				end := i + batchSize
				if end > len(instances) {
					end = len(instances)
				}
				batch := instances[i:end]
				err := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&batch).Error
				if err != nil {
					log.Fatalf("failed to bulk insert tags instances: %v", err)
				}
			}
		}

		log.Printf("%d tags added successfully\n", len(instances))
	}
}

func initializeDatabase(db *gorm.DB) {
	err := db.AutoMigrate(&TagsInstances{}, &Tags{}, &TagsCollections{}, &Collections{})
	if err != nil {
		log.Fatalf("failed to auto-migrate: %v", err)
	}

	// Check and create the Default collection if it does not exist
	collection := Collections{}
	if err := db.FirstOrCreate(&collection, Collections{Name: "Default", UserID: 0}).Error; err != nil {
		log.Fatalf("failed to check/create default collection: %v", err)
	}

	// Check and create the required tags if they do not exist
	for tagName, metadata := range TagNames {
		tag := Tags{}
		if err := db.FirstOrCreate(&tag, Tags{Name: tagName, UserID: 0}).Error; err != nil {
			log.Fatalf("failed to check/create tag '%v': %v", tagName, err)
		}
		metadata.ID = tag.ID // Store the ID back in the map
		TagNames[tagName] = metadata

		// Add tag to the Default collection if not already added
		tagsCollection := TagsCollections{}
		if err := db.FirstOrCreate(&tagsCollection, TagsCollections{TagID: tag.ID,
			CollectionID: collection.ID}).Error; err != nil {
			log.Fatalf("failed to add tag '%v' to Default collection: %v", tagName, err)
		}
	}

	log.Println("Setup completed successfully.")
}

// Assuming this is the struct that represents the result of an OHLCV query.
type OHLCV struct {
	SymbolID uint
	Datetime time.Time
	Open     float64
	High     float64
	Low      float64
	Close    float64
	Volume   float64
}

// calculateTrueRange calculates the True Range for a given day.
func calculateTrueRange(current, prev OHLCV) float64 {
	tr1 := current.High - current.Low
	tr2 := math.Abs(current.High - prev.Close)
	tr3 := math.Abs(current.Low - prev.Close)
	return math.Max(tr1, math.Max(tr2, tr3))
}

// Modified function to calculate ATR for each date per symbol
func calculateDailyATR14(ohlcvData []OHLCV) map[uint]map[time.Time]float64 {
	symbolGroups := make(map[uint][]OHLCV)
	for _, data := range ohlcvData {
		symbolGroups[data.SymbolID] = append(symbolGroups[data.SymbolID], data)
	}

	dailyATR := make(map[uint]map[time.Time]float64)
	for symbolID, data := range symbolGroups {
		// Ensure data is sorted by date in ascending order
		sort.Slice(data, func(i, j int) bool {
			return data[i].Datetime.Before(data[j].Datetime)
		})

		// Map to keep ATR values for each symbol by date
		dailyATR[symbolID] = make(map[time.Time]float64)

		trueRanges := []float64{}
		for i := range data {
			if i == 0 {
				trueRanges = append(trueRanges, data[i].High-data[i].Low) // Simplified for the first entry
			} else {
				tr := calculateTrueRange(data[i], data[i-1])
				trueRanges = append(trueRanges, tr)
			}

			// Calculate 14-day moving ATR
			if len(trueRanges) >= 14 {
				start := max(0, i-13) // Ensure we use the last 14 or fewer entries
				end := i + 1
				windowTR := trueRanges[start:end]
				sumTR := 0.0
				for _, tr := range windowTR {
					sumTR += tr
				}
				dailyATR[symbolID][data[i].Datetime] = sumTR / float64(len(windowTR))
			}
		}
	}

	return dailyATR
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func getAllDayBullTrendTagsInstances(db *gorm.DB) ([]TagsInstances, error) {
	var allData []OHLCV
	tables := bars.InvertedRTHTables()
	for tableName, symbols := range tables {
		symbolClauseParts := []string{}
		for _, symbol := range symbols {
			symbolClauseParts = append(symbolClauseParts, fmt.Sprintf("symbol_id = %d", symbol))
		}
		symbolClause := strings.Join(symbolClauseParts, " OR ")
		rows, err := db.Raw(fmt.Sprintf(`
    SELECT symbol_id, bucket AS datetime, (agg).open AS open, (agg).high AS high, 
  (agg).low AS low, (agg).close AS close, (agg).volume AS volume
    FROM %s
    WHERE %s
    ORDER BY datetime ASC
    `, tableName, symbolClause)).Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var ohlcv OHLCV
			if err = db.ScanRows(rows, &ohlcv); err != nil {
				log.Fatalf("failed to scan rows: %v", err)
			}
			allData = append(allData, ohlcv)
		}
	}
	// Calculate the 14-day ATR for each symbol and date
	atrValues := calculateDailyATR14(allData)

	var tagsInstances []TagsInstances
	for _, data := range allData {
		dateAtrMap, exists := atrValues[data.SymbolID]
		if !exists {
			continue // Skip if no ATR value is calculated for this symbol
		}
		atr, ok := dateAtrMap[data.Datetime]
		if !ok {
			continue // Skip if no ATR value is calculated for this date
		}
		heightReq := (data.High - data.Low) * 0.2
		if data.Close > data.Open &&
			(data.High-data.Low) >= atr*3/4 &&
			(data.Open-data.Low) <= heightReq &&
			(data.High-data.Close) <= heightReq {
			tagsInstances = append(tagsInstances, TagsInstances{
				Datetime: time.Date(data.Datetime.Year(), data.Datetime.Month(), data.Datetime.Day(), 16, 0, 0, 0, time.UTC), // Not really UTC but the database thinks so
				SymbolID: data.SymbolID,
			})
		}
	}
	return tagsInstances, nil
}

func getAllDayBearTrendTagsInstances(db *gorm.DB) ([]TagsInstances, error) {
	var allData []OHLCV
	tables := bars.InvertedRTHTables()
	for tableName, symbols := range tables {
		symbolClauseParts := []string{}
		for _, symbol := range symbols {
			symbolClauseParts = append(symbolClauseParts, fmt.Sprintf("symbol_id = %d", symbol))
		}
		symbolClause := strings.Join(symbolClauseParts, " OR ")
		rows, err := db.Raw(fmt.Sprintf(`
    SELECT symbol_id, bucket AS datetime, (agg).open AS open, (agg).high AS high, 
  (agg).low AS low, (agg).close AS close, (agg).volume AS volume
    FROM %s
    WHERE %s
    ORDER BY datetime ASC
    `, tableName, symbolClause)).Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var ohlcv OHLCV
			if err = db.ScanRows(rows, &ohlcv); err != nil {
				log.Fatalf("failed to scan rows: %v", err)
			}
			allData = append(allData, ohlcv)
		}
	}
	// Calculate the 14-day ATR for each symbol and date
	atrValues := calculateDailyATR14(allData)

	var tagsInstances []TagsInstances
	for _, data := range allData {
		dateAtrMap, exists := atrValues[data.SymbolID]
		if !exists {
			continue // Skip if no ATR value is calculated for this symbol
		}
		atr, ok := dateAtrMap[data.Datetime]
		if !ok {
			continue // Skip if no ATR value is calculated for this date
		}
		heightReq := (data.High - data.Low) * 0.2
		if data.Close < data.Open &&
			(data.High-data.Low) >= atr*3/4 &&
			(data.Close-data.Low) <= heightReq &&
			(data.High-data.Open) <= heightReq {
			tagsInstances = append(tagsInstances, TagsInstances{
				Datetime: time.Date(data.Datetime.Year(), data.Datetime.Month(), data.Datetime.Day(), 16, 0, 0, 0, time.UTC), // Not really UTC but the database thinks so
				SymbolID: data.SymbolID,
			})
		}
	}
	return tagsInstances, nil
}
