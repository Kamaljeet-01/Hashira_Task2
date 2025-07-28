package main

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// MenuItem represents a single item in the master menu.
type MenuItem struct {
	ItemName        string  `json:"item_name"`
	Category        string  `json:"category"`
	Calories        int     `json:"calories"`
	TasteProfile    string  `json:"taste_profile"`
	PopularityScore float64 `json:"popularity_score"`
}

// Combo represents a single meal combination in the desired output format.
type Combo struct {
	ComboID       string  `json:"combo_id"`
	Main          string  `json:"main"`
	Side          string  `json:"side"`
	Drink         string  `json:"drink"`
	CalorieCount  int     `json:"calorie_count"`
	PopularityAvg float64 `json:"popularity_score"`
	Reasoning     string  `json:"reasoning"`
}

// DailyMenu represents the combos for a single day.
type DailyMenu struct {
	Day    string  `json:"day"`
	Combos []Combo `json:"combos"`
}

// MenuPlan represents the entire 3-day (now 7-day) menu plan for JSON output.
type MenuPlan struct {
	MenuPlan []DailyMenu `json:"menu_plan"`
}

// loadMenuFromJSON reads the master menu from a JSON file.
func loadMenuFromJSON(path string) ([]MenuItem, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read menu file %s: %w", path, err)
	}
	var items []MenuItem
	err = json.Unmarshal(data, &items)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON from %s: %w", path, err)
	}
	return items, nil
}

// categorizeMenu groups menu items by their category.
func categorizeMenu(items []MenuItem) map[string][]MenuItem {
	categorized := make(map[string][]MenuItem)
	for _, item := range items {
		categorized[item.Category] = append(categorized[item.Category], item)
	}
	return categorized
}

// calculateComboMetrics calculates total calories and average popularity.
func calculateComboMetrics(main, side, drink MenuItem) (int, float64) {
	totalCalories := main.Calories + side.Calories + drink.Calories
	averagePopularity := (main.PopularityScore + side.PopularityScore + drink.PopularityScore) / 3.0
	return totalCalories, averagePopularity
}

// isValidCombo checks if a combo meets calorie and popularity criteria.
func isValidCombo(main, side, drink MenuItem, minCalories, maxCalories int, popularityTolerance float64) bool {
	totalCalories, _ := calculateComboMetrics(main, side, drink)

	if !(totalCalories >= minCalories && totalCalories <= maxCalories) {
		return false
	}

	popularityScores := []float64{main.PopularityScore, side.PopularityScore, drink.PopularityScore}
	sort.Float64s(popularityScores)
	if len(popularityScores) > 1 && (popularityScores[len(popularityScores)-1]-popularityScores[0]) > popularityTolerance {
		return false
	}

	return true
}

// generateReasoning creates a descriptive reasoning string for a combo.
func generateReasoning(main, side, drink MenuItem, totalCalories int, avgPopularity float64) string {
	tasteProfiles := make(map[string]bool)
	tasteProfiles[main.TasteProfile] = true
	tasteProfiles[side.TasteProfile] = true
	tasteProfiles[drink.TasteProfile] = true

	tasteDesc := ""
	if len(tasteProfiles) == 1 {
		for k := range tasteProfiles {
			tasteDesc = fmt.Sprintf("a %s profile", k)
		}
	} else if tasteProfiles["spicy"] {
		tasteDesc = "a spicy and mixed taste profile"
	} else if tasteProfiles["sweet"] {
		tasteDesc = "a sweet and mixed taste profile"
	} else if tasteProfiles["savory"] {
		tasteDesc = "a savory and mixed taste profile"
	} else if tasteProfiles["fresh"] {
		tasteDesc = "a fresh and mixed taste profile"
	} else {
		tasteDesc = "a mixed taste profile"
	}

	return fmt.Sprintf("This combo features %s, consists of popular choices (average popularity: %.2f), and meets the calorie target (%d kcal).",
		tasteDesc, avgPopularity, totalCalories)
}

// generateDailyCombos generates unique combos for a single day, respecting all constraints.
// It now takes the currentDayIndex and a map for 3-day combo repetition.
func generateDailyCombos(
	categorizedMenu map[string][]MenuItem,
	numCombosPerDay int,
	minCalories, maxCalories int,
	usedItemsForDay1 *map[string]bool, // Pointer to track Day 1 item uniqueness
	allGeneratedComboSignatures map[string]int, // Map: comboSignature -> lastDayIndexUsed
	currentDayIndex int, // New parameter: 0 for Mon, 1 for Tue, etc.
	globalComboCounter *int, // For generating unique combo IDs across the week
) []Combo {
	dailyCombos := []Combo{}
	currentDayUsedItems := make(map[string]bool) // Items used in combos for the current day

	mains := categorizedMenu["main"]
	sides := categorizedMenu["side"]
	drinks := categorizedMenu["drink"]

	if len(mains) == 0 || len(sides) == 0 || len(drinks) == 0 {
		log.Println("Error: Not enough items in all categories to form combos.")
		return []Combo{}
	}

	const maxAttemptsPerCombo = 5000

	for i := 0; i < numCombosPerDay; i++ {
		attempts := 0
		comboFound := false
		for attempts < maxAttemptsPerCombo {
			attempts++

			mainItem := mains[rand.Intn(len(mains))]
			sideItem := sides[rand.Intn(len(sides))]
			drinkItem := drinks[rand.Intn(len(drinks))]

			isUniqueForDay1 := true
			if usedItemsForDay1 != nil { // Only for Day 1 (index 0)
				if (*usedItemsForDay1)[mainItem.ItemName] || (*usedItemsForDay1)[sideItem.ItemName] || (*usedItemsForDay1)[drinkItem.ItemName] {
					isUniqueForDay1 = false
				}
			}

			isUniqueForCurrentDayItems := true
			if currentDayUsedItems[mainItem.ItemName] || currentDayUsedItems[sideItem.ItemName] || currentDayUsedItems[drinkItem.ItemName] {
				isUniqueForCurrentDayItems = false
			}

			itemNames := []string{mainItem.ItemName, sideItem.ItemName, drinkItem.ItemName}
			sort.Strings(itemNames)
			comboSignature := strings.Join(itemNames, "_")

			// Check 3-day repetition rule
			isUniqueWithin3Days := true
			if lastUsedDay, ok := allGeneratedComboSignatures[comboSignature]; ok {
				if currentDayIndex-lastUsedDay < 3 { // Combo used within the last 3 days
					isUniqueWithin3Days = false
				}
			}

			if isUniqueForDay1 && isUniqueForCurrentDayItems && isUniqueWithin3Days &&
				isValidCombo(mainItem, sideItem, drinkItem, minCalories, maxCalories, 0.15) {

				totalCalories, avgPopularity := calculateComboMetrics(mainItem, sideItem, drinkItem)

				*globalComboCounter++ // Increment global counter for unique ID
				combo := Combo{
					ComboID:       fmt.Sprintf("combo_%d", *globalComboCounter),
					Main:          mainItem.ItemName,
					Side:          sideItem.ItemName,
					Drink:         drinkItem.ItemName,
					CalorieCount:  totalCalories,
					PopularityAvg: math.Round(avgPopularity*100) / 100,
					Reasoning:     generateReasoning(mainItem, sideItem, drinkItem, totalCalories, avgPopularity),
				}
				dailyCombos = append(dailyCombos, combo)

				currentDayUsedItems[mainItem.ItemName] = true
				currentDayUsedItems[sideItem.ItemName] = true
				currentDayUsedItems[drinkItem.ItemName] = true

				if usedItemsForDay1 != nil {
					(*usedItemsForDay1)[mainItem.ItemName] = true
					(*usedItemsForDay1)[sideItem.ItemName] = true
					(*usedItemsForDay1)[drinkItem.ItemName] = true
				}

				allGeneratedComboSignatures[comboSignature] = currentDayIndex // Update last used day for this combo

				comboFound = true
				break
			}
		}
		if !comboFound {
			log.Printf("Warning: Could not find a unique and valid combo for slot %d on day %d after %d attempts. "+
				"This might indicate insufficient unique items or very strict constraints.\n", i+1, currentDayIndex+1, maxAttemptsPerCombo)
			break
		}
	}
	return dailyCombos
}

// generateMenuSuggestions generates a 7-day menu plan.
func generateMenuSuggestions(
	masterMenu []MenuItem,
	numDays, numCombosPerDay, minCalories, maxCalories int,
) MenuPlan {
	categorizedMenu := categorizeMenu(masterMenu)
	fullMenuPlan := MenuPlan{MenuPlan: []DailyMenu{}}

	rand.Seed(time.Now().UnixNano())

	day1OverallUsedItems := make(map[string]bool)
	// Map: comboSignature -> lastDayIndexUsed (0 for Mon, 1 for Tue, etc.)
	allGeneratedComboSignatures := make(map[string]int)
	globalComboCounter := 0 // To generate unique combo IDs across the entire week

	dayNames := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday", "Sunday"}

	for dayIndex := 0; dayIndex < numDays; dayIndex++ { // Loop for 7 days
		log.Printf("Generating menu for %s (Day %d)...\n", dayNames[dayIndex], dayIndex+1)

		var currentDayItemUniquenessTracker *map[string]bool
		if dayIndex == 0 { // Only for Monday (Day 1)
			currentDayItemUniquenessTracker = &day1OverallUsedItems
		} else {
			currentDayItemUniquenessTracker = nil
		}

		dailyCombos := generateDailyCombos(
			categorizedMenu,
			numCombosPerDay,
			minCalories, maxCalories,
			currentDayItemUniquenessTracker,
			allGeneratedComboSignatures, // Pass the map for 3-day repetition tracking
			dayIndex,                    // Pass current day index
			&globalComboCounter,         // Pass global combo counter
		)

		if len(dailyCombos) < numCombosPerDay {
			log.Printf("Note: Generated only %d out of %d combos for %s. "+
				"This might happen if constraints are too strict for the available menu items.\n",
				len(dailyCombos), numCombosPerDay, dayNames[dayIndex])
		}

		fullMenuPlan.MenuPlan = append(fullMenuPlan.MenuPlan, DailyMenu{
			Day:    dayNames[dayIndex],
			Combos: dailyCombos,
		})
	}
	return fullMenuPlan
}

// generateMenuHandler is the HTTP handler for menu generation requests.
func generateMenuHandler(w http.ResponseWriter, r *http.Request) {
	menuFilePath := "./data/master_menu.json"

	items, err := loadMenuFromJSON(menuFilePath)
	if err != nil {
		log.Printf("Error loading menu file: %v", err)
		http.Error(w, fmt.Sprintf("Unable to load menu file: %v", err), http.StatusInternalServerError)
		return
	}

	if len(items) == 0 {
		http.Error(w, "Master menu is empty or could not be loaded.", http.StatusInternalServerError)
		return
	}

	// Generate a 7-day menu plan
	menuPlan := generateMenuSuggestions(items, 7, 3, 550, 800) // numDays is now 7

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(menuPlan)
}

func main() {
	http.Handle("/", http.FileServer(http.Dir("./frontend")))
	http.HandleFunc("/generate-menu", generateMenuHandler)

	fmt.Println("✅ Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
