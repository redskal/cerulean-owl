/*
 * CeruleanOwl
 * by @sam_phisher
 *
 * The goal is to utilise Google dorking to identify users of LinkedIn
 * related to a target company in order to produce a list of employee names
 * without touching the company's infrastructure.
 *
 * These names can later be converted into a user list for further attacks.
 * This borrows a considerable amount of code from my DragonVomit Bing/Google
 * document dorker.
 */
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"

	"github.com/spf13/viper"
	customsearch "google.golang.org/api/customsearch/v1"
	"google.golang.org/api/option"
)

var (
	author  = "@sam_phisher"
	version = 0.1
	banner  string
)

type empty struct{}

func init() {
	banner = fmt.Sprintf(`            ______
           /      \
          |  Hoo?  |
     ___  /\______/
    (o,o)
    /)__)
    --"--

 CeruleanOwl - v%v
 by %s
 Identify employees on LinkedIn through dorking.
`, version, author)
}

func main() {
	//configPtr := flag.String("config", "", "Set API keys")
	targetPtr := flag.String("target", "", "The target company name to dork. E.g. \"Rootshell Security\"")
	threadCount := flag.Int("threads", 50, "Number of threads used to process results")
	queryLimit := flag.Int("limit", 10, "Limit how many pages of results you want to search. Pages contain 10 search results")

	flag.Usage = func() {
		flagSet := flag.CommandLine
		fmt.Println(banner)
		shorthand := []string{"target", "limit", "threads"} // used to maintain the ordering each run
		hints := map[string]string{                         // can't find a cleaner way of doing this. Tried reflect.TypeOf, but it's ugly for flag stuff
			"target":  "<company_name>",
			"limit":   "<query_limit>",
			"threads": "<thread_count>",
		}
		for _, name := range shorthand {
			flag := flagSet.Lookup(name)
			if flag.DefValue != "" {
				fmt.Printf("    -%-8s %-18v :: %s (Default: %s)\n", flag.Name, hints[name], flag.Usage, flag.DefValue)
			} else {
				fmt.Printf("    -%-8s %-18v :: %s\n", flag.Name, hints[name], flag.Usage)
			}
		}
		fmt.Println()
	}
	flag.Parse()

	configDir, err := getConfigDir()
	if err != nil {
		log.Fatal(err)
	}
	viper.AddConfigPath(configDir)
	viper.SetConfigName("settings")
	viper.SetConfigType("yaml")

	// set blank defaults so we can generate an empty config file if needed
	viper.SetDefault("google.cx", "")
	viper.SetDefault("google.key", "")

	err = viper.ReadInConfig()
	if err != nil {
		// create an empty config file
		_ = os.MkdirAll(configDir, os.ModePerm)
		viper.SafeWriteConfig()
		f := filepath.Join(configDir, "settings.yaml")
		log.Fatalf("configuration file generated. Set Google CX and key in %s\n", f)
	}

	googleCx := viper.GetString("google.cx")
	googleKey := viper.GetString("google.key")

	// nothing to search for or no keys?
	if *targetPtr == "" || googleCx == "" || googleKey == "" {
		flag.Usage()
		f := filepath.Join(configDir, "settings.yaml")
		log.Fatalf("no target set, or Google API keys not configured. Add API keys in: %s\n", f)
	}

	returnedNames := make(chan string, *threadCount)
	gather := make(chan string)
	tracker := make(chan empty)

	// run the dorking and send results to workers for processing...
	go func(returnedNames chan string) {
		ctx := context.Background()
		customSearchService, err := customsearch.NewService(ctx, option.WithAPIKey(googleKey))
		if err != nil {
			var e empty
			tracker <- e
			fmt.Println("ERROR:", err.Error())
			return
		}
		// I've tried to remove posts, but Google seems to selectively ignore the "-inurl" operator.
		searchQuery := fmt.Sprintf("site:linkedin.com inurl:\"/in/\" -inurl:post -inurl:dir \"at %s\" \"current\"", *targetPtr)

		// customsearch only retrieves 10 results per request, so...
		for i := 1; i < (*queryLimit * 10); i += 10 {
			resp, err := customSearchService.Cse.List().Cx(googleCx).Q(searchQuery).Start(int64(i)).Do()
			if err != nil {
				fmt.Println("ERROR:", err.Error())
				break
			}

			// no more results? let's not waste queries or compute
			if len(resp.Items) == 0 {
				break
			}

			// send results for processing
			for _, result := range resp.Items {
				returnedNames <- result.Title
			}

			// if it was less than 10 results we've probably reached the
			// end of the results and need not waste an extra query
			if len(resp.Items) < 10 {
				break
			}
		}
		var e empty
		tracker <- e
	}(returnedNames)

	// workers to process the names...
	for i := 0; i < *threadCount; i++ {
		go func(tracker chan empty, gather chan string, returnedNames chan string) {
			for name := range returnedNames {
				// if it doesn't contain "-", it's probably a false positive
				dashIndex := strings.Index(name, " - ") // spaces added to account for double-barreled names
				if dashIndex < 0 {
					continue
				}
				cleanName := strings.TrimSpace(name[:dashIndex])
				gather <- cleanName
			}
			var e empty
			tracker <- e
		}(tracker, gather, returnedNames)
	}

	// output the results...
	go func() {
		var dedupedResults []string
		for r := range gather {
			if !slices.Contains(dedupedResults, r) {
				dedupedResults = append(dedupedResults, r)
				fmt.Println(r)
			}
		}
		var e empty
		tracker <- e
	}()

	// wait for workers and output to complete
	<-tracker // customsearch
	close(returnedNames)
	for i := 0; i < *threadCount; i++ {
		<-tracker // workers
	}
	close(gather)
	<-tracker // output
}

func getConfigDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	var configDir string
	switch runtime.GOOS {
	case "windows":
		configDir = filepath.Join(homeDir, "CeruleanOwl")
	default:
		configDir = filepath.Join(homeDir, ".CeruleanOwl")
	}

	return configDir, nil
}
