package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/adrg/strutil"
	"github.com/adrg/strutil/metrics"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	configFlag := flag.String("config", "", "Path to the configuration file")
	flag.StringVar(configFlag, "c", "", "Path to the configuration file (shorthand)")

	// Parse command-line flags
	flag.Parse()

	var configFile Path
	if *configFlag != "" {
		configFile = Path(*configFlag)
	}

	config, err := LoadConfig(configFile)
	if err != nil {
		panic(err)
	}

	// testMatching()
	// os.Exit(0)

	err = runMediaSync(*config)
	if err != nil {
		panic(err)
	}
	// flag.Parse()

	// if flag.NArg() < 1 {
	// 	help()
	// 	os.Exit(1)
	// }

	// switch command := flag.Arg(0); command {
	// case "sync":
	// 	sync()
	// case "update":
	// 	updateMetadata()
	// default:
	// 	fmt.Printf("Unknown command: %s\n", command)
	// 	help()
	// 	os.Exit(1)
	// }
}

func testMatching() {

	ms := map[string]strutil.StringMetric{
		"Hamming": &metrics.Hamming{CaseSensitive: false},
		"Jaccard": &metrics.Jaccard{CaseSensitive: false,
			NgramSize: 2},
		"Jaro":        &metrics.Jaro{CaseSensitive: false},
		"JaroWinkler": &metrics.JaroWinkler{CaseSensitive: false},
		"Levenshtein": &metrics.Levenshtein{CaseSensitive: false,
			InsertCost:  1,
			DeleteCost:  1,
			ReplaceCost: 1},
		// "MatchMismatch": &metrics.MatchMismatch{},
		"OverlapCoefficient": &metrics.OverlapCoefficient{CaseSensitive: false,
			NgramSize: 2},
		"SmithWatermanGotoh": &metrics.SmithWatermanGotoh{CaseSensitive: false,
			GapPenalty: -0.5,
			Substitution: metrics.MatchMismatch{
				Match:    1,
				Mismatch: -2,
			}},
		"SorensenDice": &metrics.SorensenDice{CaseSensitive: false,
			NgramSize: 2},
		// "Substitution": &metrics.Substitution{},
	}

	for k, metric := range ms {
		similarity := strutil.Similarity("Один дома 2", "Один Дома", metric)
		similarity2 := strutil.Similarity("Один дома 2", "Дома Один", metric)

		similarity3 := strutil.Similarity("Cat Watch (2014)", "Cat Watch 2014: The New Horizon Experiment (2014)", metric)

		similarity4 := strutil.Similarity("BBC: Секреты собак", "Секреты собак", metric)
		similarity5 := strutil.Similarity("Собачий секрет", "Секреты собак", metric)
		similarity6 := strutil.Similarity("Секреты", "Секреты собак", metric)

		fmt.Printf("%s: %.2f/%.2f %.2f %.2f/%.2f/%.2f\n", k, similarity, similarity2, similarity3, similarity4, similarity5, similarity6)
	}
	os.Exit(0)

}

func help() {
	commandDescriptions := map[string]string{
		"sync":   "Sync directory contents with the database",
		"update": "Update metadata",
	}

	fmt.Printf("Usage: %s <command>\n", os.Args[0])
	fmt.Println("Commands:")
	for cmd, desc := range commandDescriptions {
		fmt.Printf("  %s: %s\n", cmd, desc)
	}
}
