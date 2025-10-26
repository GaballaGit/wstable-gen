package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"unicode"

	"github.com/lpernett/godotenv"
)

type Workshop struct {
	Name     string
	Team     string
	Semester string
	Link     string
}

type WorkshopTable map[string]map[string][]Workshop // team -> semester -> workshops

// Maps for normalization
var semesterShortcuts = map[string]string{
	"f24": "fa24",
	"s25": "sp25",
	"f25": "fa25",
	"s26": "sp26",
}

var (
	semesters = []string{"fa24", "sp25", "fa25"}
	teams     = []string{"ai", "algo", "design", "dev", "gamedev", "general", "icpc", "nodebuds", "oss"}
)

func main() {
	// Getting da file path

	godotenv.Load()

	bp := os.Getenv("ABS_PATH")
	fp := bp + "/src/lib/public/links/links.json"

	data, err := os.ReadFile(fp)
	if err != nil {
		panic(err)
	}

	var links map[string]string
	if err := json.Unmarshal(data, &links); err != nil {
		panic(err)
	}

	// Precompile regex patterns
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`^(\w+)/([\w+-]+)-([A-Za-z]{1,2}\d{2})$`), // Pattern 1: team/workshop-sem
		regexp.MustCompile(`^(\w+)/([A-Za-z]{1,2}\d{2})-([\w+-]+)$`), // Pattern 2: team/sem-workshop
		regexp.MustCompile(`^(\w+)-([\w+-]+)-([A-Za-z]{1,2}\d{2})$`), // Pattern 3: team-workshop-sem
		regexp.MustCompile(`^(\w+)-([A-Za-z]{1,2}\d{2})-([\w+-]+)$`), // Pattern 4: team-sem-workshop
		regexp.MustCompile(`^([A-Za-z]{1,2}\d{2})-(\w+)-([\w+-]+)$`), // Pattern 5: sem-team-workshop
	}

	table := make(WorkshopTable)
	for _, t := range teams {
		table[t] = make(map[string][]Workshop)
		for _, sem := range semesters {
			table[t][sem] = []Workshop{}
		}
	}

	// I am going to try waitgroups for concurrency
	var wg sync.WaitGroup
	for key, link := range links {
		var w Workshop

		// I LOVE GO 1.25 YAYAY <3
		wg.Go(func() {
			parseLink(w, &table, patterns, key, link)
		})

	}

	wg.Wait()
	// Output in TypeScript format
	fmt.Println("export var currentTable: Tables = {")
	for _, team := range teams {
		fmt.Printf("\t%s: {\n\t\tworkshops: {\n", team)
		for i, sem := range semesters {
			fmt.Printf("\t\t\t%s: [\n", sem)
			for _, w := range table[team][sem] {
				fmt.Printf("\t\t\t\t{ name: \"%s\", team: \"%s\", semester: \"%s\", link: \"%s\" },\n",
					escape(w.Name), w.Team, w.Semester, w.Link)
			}
			fmt.Printf("\t\t\t]%s\n", comma(i, len(semesters)))
		}
		fmt.Printf("\t\t}\n\t}%s\n", comma(indexOf(team, teams), len(teams)))
	}
	fmt.Println("}")
}

func parseLink(w Workshop, table *WorkshopTable, patterns []*regexp.Regexp, key, link string) {
	matched := false

	for _, re := range patterns {
		m := re.FindStringSubmatch(key)
		if len(m) == 0 {
			continue
		}

		switch {
		case re == patterns[0]: // team/workshop-sem
			w = Workshop{Name: m[2], Team: m[1], Semester: normalizeSemester(m[3]), Link: link}
		case re == patterns[1]: // team/sem-workshop
			w = Workshop{Name: (m[3]), Team: m[1], Semester: normalizeSemester(m[2]), Link: link}
		case re == patterns[2]: // team-workshop-sem
			w = Workshop{Name: (m[2]), Team: m[1], Semester: normalizeSemester(m[3]), Link: link}
		case re == patterns[3]: // team-sem-workshop
			w = Workshop{Name: (m[3]), Team: m[1], Semester: normalizeSemester(m[2]), Link: link}
		case re == patterns[4]: // sem-team-workshop
			w = Workshop{Name: (m[3]), Team: m[2], Semester: normalizeSemester(m[1]), Link: link}
		}

		if isValidTeam(w.Team) && isValidSemester(w.Semester) {
			err := nameManager(&w)
			if err != nil {
				continue
			}
			(*table)[w.Team][w.Semester] = append((*table)[w.Team][w.Semester], w)
		}
		matched = true
		break
	}

	if !matched {
		fmt.Fprintf(os.Stderr, "WARN: Could not parse key: %s\n", key)
	}
}

func normalizeSemester(s string) string {
	s = strings.ToLower(s)
	if v, ok := semesterShortcuts[s]; ok {
		return v
	}
	return s
}

func isValidTeam(t string) bool {
	for _, x := range teams {
		if t == x {
			return true
		}
	}
	return false
}

func isValidSemester(s string) bool {
	for _, x := range semesters {
		if s == x {
			return true
		}
	}
	return false
}

func escape(s string) string {
	s = strings.ReplaceAll(s, "\"", "\\\"")
	s = strings.ReplaceAll(s, "&#39;", "'")
	return s
}

func comma(i, n int) string {
	if i == n-1 {
		return ""
	}
	return ","
}

func indexOf[T comparable](x T, list []T) int {
	for i, v := range list {
		if v == x {
			return i
		}
	}
	return -1
}

func nameManager(w *Workshop) error {
	if !strings.Contains(w.Link, "docs.google.com/presentation") {
		w.Name = nameNoLink(w.Name)
		return nil
	}

	resp, err := http.Get(w.Link)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}

	re := regexp.MustCompile(`(?i)<title>(.*?)</title>`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		fmt.Errorf("no title found")
		return nil
	}

	title := strings.TrimSpace(matches[1])
	title = cleanGoogleTitle(title)

	if strings.Contains(title, "Sign in") {
		fmt.Errorf("link not publicly accessible")
		return nil
	}

	if strings.Contains(title, "Page Not Found") {
		return fmt.Errorf("Workshop unavaliable")
	}

	w.Name = title
	return nil
}

func cleanGoogleTitle(title string) string {
	suffixes := []string{
		" - Google Slides",
		" - Google Docs",
		" - Google Drive",
		"- Google Slides",
		"- Google Docs",
	}
	for _, s := range suffixes {
		title = strings.TrimSuffix(title, s)
	}

	title = strings.ReplaceAll(title, "&amp;", "&")
	return strings.TrimSpace(title)
}

func nameNoLink(name string) string {
	newName := strings.ReplaceAll(name, "-", " ")
	nNf := strings.Fields(newName)

	var nN []byte
	for j, elm := range nNf {
		if j != 0 {
			nN = append(nN, byte(' '))
		}

		for i, chr := range elm {
			r := rune(chr)
			if unicode.IsLetter(r) && i == 0 {
				nN = append(nN, byte(unicode.ToUpper(r)))
			} else {
				nN = append(nN, byte(r))
			}
		}

	}
	return string(nN)
}
