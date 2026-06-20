package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

func main() {
	contestDir := os.Getenv("TCFORGE_CONTEST_DIR")
	if contestDir == "" {
		contestDir = "/contest"
	}

	dbPath := filepath.Join(contestDir, ".tcforge", "db.sqlite")

	var db *sql.DB
	for {
		var err error
		db, err = sql.Open("sqlite", dbPath+"?_busy_timeout=5000")
		if err == nil {
			if _, err = db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err == nil {
				break
			}
		}
		log.Println("waiting for db:", err)
		time.Sleep(2 * time.Second)
	}
	defer db.Close()

	log.Println("judge worker ready")

	for {
		if err := processNext(db, contestDir); err != nil {
			log.Printf("error: %v", err)
		}
		time.Sleep(time.Second)
	}
}

// contestScoring reads the scoring type from tcforge.yaml.
func contestScoring(contestDir string) string {
	data, err := os.ReadFile(filepath.Join(contestDir, "tcforge.yaml"))
	if err != nil {
		return "ioi"
	}
	// simple scan — avoid importing yaml to keep binary small
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "scoring:") {
			v := strings.TrimSpace(strings.TrimPrefix(line, "scoring:"))
			if v == "icpc" {
				return "icpc"
			}
		}
	}
	return "ioi"
}

// problemConfig mirrors the tcframe/Judgels config.json format.
type problemConfig struct {
	TestGroups [][]int `json:"test_groups"` // [i] = subtask IDs that group i+1 belongs to
	Points     []int   `json:"points"`      // [j] = points for subtask j+1
}

func loadProblemConfig(problemDir string) *problemConfig {
	data, err := os.ReadFile(filepath.Join(problemDir, "config.json"))
	if err != nil {
		return nil
	}
	var cfg problemConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil
	}
	if len(cfg.TestGroups) == 0 || len(cfg.Points) == 0 {
		return nil
	}
	return &cfg
}

type submission struct {
	ID          int
	ProblemPath string
	Language    string
	Code        string
	TimeLimit   int
	MemoryLimit int
}

func processNext(db *sql.DB, contestDir string) error {
	var sub submission
	err := db.QueryRow(`
		SELECT s.id, p.path, s.language, s.code, p.time_limit, p.memory_limit
		FROM submissions s
		JOIN problems p ON s.problem_id = p.id
		WHERE s.status = 'queued'
		ORDER BY s.submitted_at ASC
		LIMIT 1
	`).Scan(&sub.ID, &sub.ProblemPath, &sub.Language, &sub.Code, &sub.TimeLimit, &sub.MemoryLimit)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("query: %w", err)
	}

	log.Printf("judging submission %d (problem=%s lang=%s)", sub.ID, sub.ProblemPath, sub.Language)
	db.Exec("UPDATE submissions SET status='judging' WHERE id=?", sub.ID)

	scoring := contestScoring(contestDir)
	finalVerdict, score, maxTimeMs, subtaskResults := judge(db, sub, contestDir, scoring)

	db.Exec("UPDATE submissions SET status='done', verdict=?, score=?, time_ms=? WHERE id=?",
		finalVerdict, score, maxTimeMs, sub.ID)

	for _, s := range subtaskResults {
		db.Exec(`INSERT INTO subtask_scores (submission_id, subtask_num, verdict, score, max_score)
			VALUES (?,?,?,?,?)`,
			sub.ID, s.subtaskNum, s.verdict, s.score, s.maxScore)
	}

	log.Printf("submission %d: %s score=%d/%d time=%dms", sub.ID, finalVerdict, score, totalMaxScore(subtaskResults), maxTimeMs)
	return nil
}

func totalMaxScore(sr []subtaskScoreResult) int {
	t := 0
	for _, s := range sr {
		t += s.maxScore
	}
	if t == 0 {
		return 100
	}
	return t
}

type tcVerdict struct {
	testCase string
	verdict  string
	timeMs   int
	memKb    int
	groupNum int
}

type subtaskScoreResult struct {
	subtaskNum int
	verdict    string
	score      int
	maxScore   int
}

var (
	reGroupTC  = regexp.MustCompile(`_(\d+)_(\d+)$`)
	reSampleTC = regexp.MustCompile(`_sample_`)
)

func parseGroupNum(base string) int {
	if reSampleTC.MatchString(base) {
		return -1 // sample — skip scoring
	}
	if m := reGroupTC.FindStringSubmatch(base); m != nil {
		n, _ := strconv.Atoi(m[1])
		return n
	}
	return 0 // no group structure
}

func writeVerdict(db *sql.DB, subID int, tc tcVerdict) {
	db.Exec(`INSERT INTO verdicts (submission_id, test_case, verdict, time_ms, memory_kb, group_num)
		VALUES (?,?,?,?,?,?)`,
		subID, tc.testCase, tc.verdict, tc.timeMs, tc.memKb, tc.groupNum)
}

func judge(db *sql.DB, sub submission, contestDir, scoring string) (
	finalVerdict string, score, maxTimeMs int, subtaskResults []subtaskScoreResult,
) {
	problemDir := filepath.Join(contestDir, sub.ProblemPath)
	tcDir := filepath.Join(problemDir, "tc")

	inFiles, err := filepath.Glob(filepath.Join(tcDir, "*.in"))
	if err != nil || len(inFiles) == 0 {
		return "IE", 0, 0, nil
	}
	sort.Strings(inFiles)

	tmpDir, err := os.MkdirTemp("", "tcforge-*")
	if err != nil {
		return "IE", 0, 0, nil
	}
	defer os.RemoveAll(tmpDir)

	binPath, err := compile(sub.Language, sub.Code, tmpDir)
	if err != nil {
		log.Printf("CE: %v", err)
		return "CE", 0, 0, nil
	}

	groupResults := map[int][]string{}
	failedGroups := map[int]bool{}
	var lastVerdict string

	for _, inFile := range inFiles {
		base := strings.TrimSuffix(filepath.Base(inFile), ".in")
		groupNum := parseGroupNum(base)
		if groupNum == -1 {
			continue // skip samples
		}

		// ICPC: stop on first failure
		if scoring == "icpc" && lastVerdict != "" && lastVerdict != "AC" {
			break
		}

		// IOI: skip remaining TCs in a failed group
		if scoring != "icpc" && failedGroups[groupNum] {
			continue
		}

		v, tMs := runCase(sub.Language, binPath, inFile,
			strings.TrimSuffix(inFile, ".in")+".out", sub.TimeLimit)

		if tMs > maxTimeMs {
			maxTimeMs = tMs
		}
		lastVerdict = v

		// Write verdict to DB immediately so frontend sees live progress
		writeVerdict(db, sub.ID, tcVerdict{
			testCase: base, verdict: v, timeMs: tMs, groupNum: groupNum,
		})

		groupResults[groupNum] = append(groupResults[groupNum], v)
		if v != "AC" {
			failedGroups[groupNum] = true
		}
	}

	if scoring == "icpc" {
		for _, vs := range groupResults {
			for _, v := range vs {
				if v != "AC" {
					return v, 0, maxTimeMs, nil
				}
			}
		}
		return "AC", 100, maxTimeMs, nil
	}

	// IOI scoring
	cfg := loadProblemConfig(problemDir)
	if cfg != nil {
		return scoreIOIWithConfig(groupResults, maxTimeMs, cfg)
	}
	return scoreIOIGroupEqual(groupResults, maxTimeMs)
}

// scoreIOIWithConfig: subtask i passes iff ALL groups listing i are fully AC.
func scoreIOIWithConfig(
	groupResults map[int][]string, maxTimeMs int, cfg *problemConfig,
) (finalVerdict string, score, _ int, subtaskResults []subtaskScoreResult) {
	for s := 1; s <= len(cfg.Points); s++ {
		subtaskAC := true
		for gIdx, subtasks := range cfg.TestGroups {
			groupNum := gIdx + 1
			for _, sub := range subtasks {
				if sub == s {
					for _, v := range groupResults[groupNum] {
						if v != "AC" {
							subtaskAC = false
						}
					}
				}
			}
		}
		pts, verdict := 0, "WA"
		if subtaskAC {
			pts = cfg.Points[s-1]
			verdict = "AC"
		}
		score += pts
		subtaskResults = append(subtaskResults, subtaskScoreResult{
			subtaskNum: s, verdict: verdict, score: pts, maxScore: cfg.Points[s-1],
		})
	}
	finalVerdict = "AC"
	for _, sr := range subtaskResults {
		if sr.verdict != "AC" {
			finalVerdict = "WA"
			break
		}
	}
	return
}

// scoreIOIGroupEqual: treat each group as equal-weight subtask when no config.json.
func scoreIOIGroupEqual(
	groupResults map[int][]string, maxTimeMs int,
) (finalVerdict string, score, _ int, subtaskResults []subtaskScoreResult) {
	groups := []int{}
	for g := range groupResults {
		groups = append(groups, g)
	}
	sort.Ints(groups)

	if len(groups) == 0 {
		return "IE", 0, maxTimeMs, nil
	}

	// No group structure — all or nothing
	if len(groups) == 1 && groups[0] == 0 {
		for _, v := range groupResults[0] {
			if v != "AC" {
				return v, 0, maxTimeMs, nil
			}
		}
		return "AC", 100, maxTimeMs, nil
	}

	pointsPerGroup := 100 / len(groups)
	finalVerdict = "AC"
	for i, g := range groups {
		pts := pointsPerGroup
		if i == len(groups)-1 {
			pts = 100 - pointsPerGroup*(len(groups)-1)
		}
		groupAC := true
		for _, v := range groupResults[g] {
			if v != "AC" {
				groupAC = false
			}
		}
		s, verdict := 0, "WA"
		if groupAC {
			s = pts
			verdict = "AC"
		} else {
			finalVerdict = "WA"
		}
		subtaskResults = append(subtaskResults, subtaskScoreResult{
			subtaskNum: g, verdict: verdict, score: s, maxScore: pts,
		})
	}
	return
}

func compile(lang, code, dir string) (string, error) {
	switch lang {
	case "cpp17", "cpp20":
		src := filepath.Join(dir, "solution.cpp")
		bin := filepath.Join(dir, "solution")
		if err := os.WriteFile(src, []byte(code), 0644); err != nil {
			return "", err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "g++", "-O2", "-std=c++20", "-o", bin, src).CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("%s", out)
		}
		return bin, nil

	case "python3":
		src := filepath.Join(dir, "solution.py")
		if err := os.WriteFile(src, []byte(code), 0644); err != nil {
			return "", err
		}
		return src, nil

	default:
		return "", fmt.Errorf("unsupported language: %s", lang)
	}
}

func runCase(lang, binPath, inFile, outFile string, timeLimitSec int) (verdict string, timeMs int) {
	expected, err := os.ReadFile(outFile)
	if err != nil {
		return "IE", 0
	}
	input, err := os.ReadFile(inFile)
	if err != nil {
		return "IE", 0
	}

	ctx, cancel := context.WithTimeout(context.Background(),
		time.Duration(timeLimitSec)*time.Second+500*time.Millisecond)
	defer cancel()

	var cmd *exec.Cmd
	switch lang {
	case "cpp17", "cpp20":
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", "ulimit -s unlimited && "+binPath)
	case "python3":
		cmd = exec.CommandContext(ctx, "python3", binPath)
	default:
		return "IE", 0
	}

	var stdout bytes.Buffer
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = &stdout

	start := time.Now()
	err = cmd.Run()
	elapsed := int(time.Since(start).Milliseconds())

	if ctx.Err() == context.DeadlineExceeded {
		return "TLE", elapsed
	}
	if err != nil {
		return "RTE", elapsed
	}
	if normalize(stdout.String()) == normalize(string(expected)) {
		return "AC", elapsed
	}
	return "WA", elapsed
}

func normalize(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n\r"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t\r")
	}
	return strings.Join(lines, "\n")
}
