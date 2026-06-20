package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

var judgeIsPostgres bool

func rebind(q string) string {
	if !judgeIsPostgres {
		return q
	}
	n := 0
	var b strings.Builder
	for _, r := range q {
		if r == '?' {
			n++
			fmt.Fprintf(&b, "$%d", n)
		} else {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func main() {
	contestDir := os.Getenv("TCFORGE_CONTEST_DIR")
	if contestDir == "" {
		contestDir = "/contest"
	}

	var db *sql.DB
	if os.Getenv("DB_TYPE") == "psql" {
		judgeIsPostgres = true
		dbURL := os.Getenv("DATABASE_URL")
		if dbURL == "" {
			log.Fatal("DB_TYPE=psql requires DATABASE_URL to be set")
		}
		for {
			var err error
			db, err = sql.Open("pgx", dbURL)
			if err == nil {
				if err = db.Ping(); err == nil {
					break
				}
			}
			log.Println("waiting for db:", err)
			time.Sleep(2 * time.Second)
		}
		// Wait for the API to finish migrating (submissions table must exist)
		for {
			rows, err := db.Query("SELECT 1 FROM submissions LIMIT 0")
			if err == nil {
				rows.Close()
				break
			}
			log.Println("waiting for schema:", err)
			time.Sleep(2 * time.Second)
		}
		log.Println("judge: using postgres")
	} else {
		dbPath := os.Getenv("TCFORGE_DB_PATH")
		if dbPath == "" {
			dbPath = filepath.Join(contestDir, ".tcforge", "db.sqlite")
		}
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
		log.Println("judge: using sqlite at", dbPath)
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

func processNext(db *sql.DB, contestDir string) (retErr error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("PANIC in processNext: %v", r)
			retErr = fmt.Errorf("panic: %v", r)
		}
	}()

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
	db.Exec(rebind("UPDATE submissions SET status='judging' WHERE id=?"), sub.ID)

	scoring := contestScoring(contestDir)
	finalVerdict, score, maxTimeMs, subtaskResults := judge(db, sub, contestDir, scoring)

	db.Exec(rebind("UPDATE submissions SET status='done', verdict=?, score=?, time_ms=?, graded_at=CURRENT_TIMESTAMP WHERE id=?"),
		finalVerdict, score, maxTimeMs, sub.ID)

	for _, s := range subtaskResults {
		db.Exec(rebind(`INSERT INTO subtask_scores (submission_id, subtask_num, verdict, score, max_score)
			VALUES (?,?,?,?,?)`),
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

// tcScore carries the grading result of one test case, matching tcframe's TestCaseVerdict model.
// For OK verdicts, exactly one of absPoints/isPct is meaningful.
type tcScore struct {
	verdict   string  // "AC", "WA", "OK", "RTE", "TLE", "IE"
	absPoints float64 // OK <n>  — absolute points
	isPct     bool    // true when OK <n>%
	pct       float64 // OK <n>% — percentage (0–100)
}

// pointsFraction converts tcScore to a 0–1 value for DB storage / display.
// For OK-absolute, stores the raw point value (caller must know the context).
func (s tcScore) pointsFraction() float64 {
	switch s.verdict {
	case "AC":
		return 1.0
	case "OK":
		if s.isPct {
			return s.pct / 100.0
		}
		return s.absPoints // raw absolute — not normalised
	default:
		return 0.0
	}
}

type tcVerdict struct {
	testCase       string
	verdict        string
	timeMs         int
	memKb          int
	groupNum       int
	pointsFraction float64
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
	db.Exec(rebind(`INSERT INTO verdicts (submission_id, test_case, verdict, time_ms, memory_kb, group_num, points_fraction)
		VALUES (?,?,?,?,?,?,?)`),
		subID, tc.testCase, tc.verdict, tc.timeMs, tc.memKb, tc.groupNum, tc.pointsFraction)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseCheckerVerdict parses scorer or communicator output.
//
// tcframe verdict format (matches TestCaseVerdictParser.hpp):
//
//	Line 1: AC | WA | OK
//	Line 2: (only for OK) <points> or <pct>%
//
// Scorer writes verdict to stdout; communicator writes to stderr.
func parseCheckerVerdict(output string) tcScore {
	lines := strings.Split(strings.TrimRight(output, "\n\r"), "\n")
	if len(lines) == 0 {
		return tcScore{verdict: "WA"}
	}
	first := strings.TrimSpace(lines[0])
	switch first {
	case "AC":
		return tcScore{verdict: "AC"}
	case "WA":
		return tcScore{verdict: "WA"}
	case "OK":
		if len(lines) < 2 {
			return tcScore{verdict: "WA"} // malformed — treat as WA
		}
		val := strings.TrimSpace(strings.Fields(lines[1])[0])
		if strings.HasSuffix(val, "%") {
			pct, err := strconv.ParseFloat(strings.TrimSuffix(val, "%"), 64)
			if err == nil {
				return tcScore{verdict: "OK", isPct: true, pct: pct}
			}
		} else {
			pts, err := strconv.ParseFloat(val, 64)
			if err == nil {
				return tcScore{verdict: "OK", absPoints: pts}
			}
		}
	}
	return tcScore{verdict: "WA"}
}

// effectivePts returns the effective score for one TC under MinAggregator semantics.
//
// tcframe MinAggregator (used when hasSubtasks=true):
//   - AC          → subtaskMax (no constraint)
//   - OK <pts>    → pts absolute
//   - OK <pct>%   → pct * subtaskMax / 100
//   - WA/RTE/...  → 0
func (s tcScore) effectivePts(subtaskMax float64) float64 {
	switch s.verdict {
	case "AC":
		return subtaskMax
	case "OK":
		if s.isPct {
			return s.pct * subtaskMax / 100.0
		}
		return s.absPoints
	default:
		return 0
	}
}

func judge(db *sql.DB, sub submission, contestDir, scoring string) (
	finalVerdict string, score, maxTimeMs int, subtaskResults []subtaskScoreResult,
) {
	problemDir := filepath.Join(contestDir, sub.ProblemPath)
	tcDir := filepath.Join(problemDir, "tc")

	scorerPath := filepath.Join(problemDir, "scorer")
	communicatorPath := filepath.Join(problemDir, "communicator")
	isInteractive := fileExists(communicatorPath)
	hasCustomScorer := !isInteractive && fileExists(scorerPath)

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

	// groupScores[groupNum] = per-TC scores in the order they were run.
	groupScores := map[int][]tcScore{}
	// failedGroups: set when a TC produces a zero-effective-points verdict (WA/RTE/TLE).
	// Safe to skip remaining TCs in that group — MinAggregator/SumAggregator can't improve.
	failedGroups := map[int]bool{}
	var icpcFailVerdict string // first non-AC verdict for ICPC result

	for _, inFile := range inFiles {
		base := strings.TrimSuffix(filepath.Base(inFile), ".in")
		groupNum := parseGroupNum(base)
		if groupNum == -1 {
			continue // skip samples
		}

		// ICPC: stop after first failure
		if scoring == "icpc" && icpcFailVerdict != "" {
			break
		}

		// Skip remaining TCs in a zero-effective group (WA/RTE/TLE).
		// OK verdicts don't trigger this — remaining TCs may lower the min further.
		if scoring != "icpc" && failedGroups[groupNum] {
			continue
		}

		var ts tcScore
		var tMs int

		if isInteractive {
			ts, tMs = runCaseInteractive(sub.Language, binPath, inFile, communicatorPath, tmpDir, sub.TimeLimit, sub.MemoryLimit)
		} else {
			outFile := strings.TrimSuffix(inFile, ".in") + ".out"
			checkerArg := ""
			if hasCustomScorer {
				checkerArg = scorerPath
			}
			ts, tMs = runCase(sub.Language, binPath, inFile, outFile, checkerArg, tmpDir, sub.TimeLimit, sub.MemoryLimit)
		}

		if tMs > maxTimeMs {
			maxTimeMs = tMs
		}

		if scoring == "icpc" && ts.verdict != "AC" && icpcFailVerdict == "" {
			icpcFailVerdict = ts.verdict
		}

		writeVerdict(db, sub.ID, tcVerdict{
			testCase:       base,
			verdict:        ts.verdict,
			timeMs:         tMs,
			groupNum:       groupNum,
			pointsFraction: ts.pointsFraction(),
		})

		groupScores[groupNum] = append(groupScores[groupNum], ts)

		// Only mark group as failed for zero-contribution verdicts (not OK).
		if ts.verdict != "AC" && ts.verdict != "OK" {
			failedGroups[groupNum] = true
		}
	}

	if scoring == "icpc" {
		if icpcFailVerdict != "" {
			return icpcFailVerdict, 0, maxTimeMs, nil
		}
		return "AC", 100, maxTimeMs, nil
	}

	// IOI scoring
	cfg := loadProblemConfig(problemDir)
	if cfg != nil {
		return scoreIOIWithConfig(groupScores, maxTimeMs, cfg)
	}
	return scoreIOIGroupEqual(groupScores, maxTimeMs)
}

// worstNonACVerdict returns the most notable failing verdict among a set of TCs:
// TLE > RTE > WA, so callers can distinguish why a subtask scored zero.
func worstNonACVerdict(tcs []tcScore) string {
	worst := "WA"
	for _, tc := range tcs {
		switch tc.verdict {
		case "TLE":
			return "TLE"
		case "RTE":
			worst = "RTE"
		}
	}
	return worst
}

// scoreIOIWithConfig uses tcframe's MinAggregator semantics:
// subtask score = min(effectivePts(tc, subtaskMax)) across all TCs in the subtask.
//
// This naturally handles both the all-or-nothing case (AC/WA only) and partial
// credit (OK <pts>), matching how tcframe's grader aggregates results.
func scoreIOIWithConfig(
	groupScores map[int][]tcScore, maxTimeMs int, cfg *problemConfig,
) (finalVerdict string, score, _ int, subtaskResults []subtaskScoreResult) {
	for s := 1; s <= len(cfg.Points); s++ {
		maxPts := float64(cfg.Points[s-1])
		subtaskScore := maxPts // start at maximum; each TC can only lower this
		hasTCs := false
		var subtaskTCs []tcScore

		for gIdx, subtasks := range cfg.TestGroups {
			groupNum := gIdx + 1
			for _, sub := range subtasks {
				if sub != s {
					continue
				}
				for _, tc := range groupScores[groupNum] {
					hasTCs = true
					subtaskTCs = append(subtaskTCs, tc)
					eff := tc.effectivePts(maxPts)
					if eff < subtaskScore {
						subtaskScore = eff
					}
				}
			}
		}

		if !hasTCs {
			subtaskScore = maxPts // no TCs for this subtask → don't penalise
		}

		pts := int(math.Round(subtaskScore))
		verdict := worstNonACVerdict(subtaskTCs)
		if pts >= cfg.Points[s-1] {
			verdict = "AC"
		} else if pts > 0 {
			verdict = "OK"
		}
		score += pts
		subtaskResults = append(subtaskResults, subtaskScoreResult{
			subtaskNum: s, verdict: verdict, score: pts, maxScore: cfg.Points[s-1],
		})
	}
	finalVerdict = "AC"
	for _, sr := range subtaskResults {
		switch sr.verdict {
		case "TLE":
			finalVerdict = "TLE"
		case "RTE":
			if finalVerdict != "TLE" {
				finalVerdict = "RTE"
			}
		case "WA", "OK":
			if finalVerdict == "AC" {
				finalVerdict = "WA"
			}
		}
	}
	return
}

// scoreIOIGroupEqual uses tcframe's SumAggregator semantics when no config.json:
// each TC earns its share of the group's points (equal weight per TC);
// OK <pts> contributes absolute points; OK <pct>% contributes pct% of the TC share.
func scoreIOIGroupEqual(
	groupScores map[int][]tcScore, maxTimeMs int,
) (finalVerdict string, score, _ int, subtaskResults []subtaskScoreResult) {
	groups := []int{}
	for g := range groupScores {
		groups = append(groups, g)
	}
	sort.Ints(groups)

	if len(groups) == 0 {
		return "IE", 0, maxTimeMs, nil
	}

	// No group structure (all TCs in group 0) — single pool with SumAggregator.
	if len(groups) == 1 && groups[0] == 0 {
		pts := sumGroupScore(groupScores[0], 100)
		if pts >= 100 {
			return "AC", 100, maxTimeMs, nil
		}
		return "WA", int(math.Round(pts)), maxTimeMs, nil
	}

	pointsPerGroup := 100 / len(groups)
	finalVerdict = "AC"
	for i, g := range groups {
		maxPts := pointsPerGroup
		if i == len(groups)-1 {
			maxPts = 100 - pointsPerGroup*(len(groups)-1)
		}
		tcs := groupScores[g]
		pts := sumGroupScore(tcs, float64(maxPts))
		earned := int(math.Round(pts))
		verdict := worstNonACVerdict(tcs)
		if earned >= maxPts {
			verdict = "AC"
		} else if earned > 0 {
			verdict = "OK"
		}
		if verdict != "AC" {
			switch {
			case verdict == "TLE" || finalVerdict == "TLE":
				finalVerdict = "TLE"
			case verdict == "RTE" || finalVerdict == "RTE":
				finalVerdict = "RTE"
			default:
				finalVerdict = "WA"
			}
		}
		score += earned
		subtaskResults = append(subtaskResults, subtaskScoreResult{
			subtaskNum: g, verdict: verdict, score: earned, maxScore: maxPts,
		})
	}
	return
}

// sumGroupScore applies SumAggregator over the TCs in one group.
// tcShare = groupMax / len(tcs); each AC earns tcShare; OK <pts> earns pts absolute;
// OK <pct>% earns pct * tcShare / 100.
func sumGroupScore(tcs []tcScore, groupMax float64) float64 {
	n := float64(len(tcs))
	if n == 0 {
		return groupMax
	}
	tcShare := groupMax / n
	total := 0.0
	for _, tc := range tcs {
		switch tc.verdict {
		case "AC":
			total += tcShare
		case "OK":
			if tc.isPct {
				total += tc.pct * tcShare / 100.0
			} else {
				total += tc.absPoints
			}
		}
	}
	return total
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

func runCase(lang, binPath, inFile, outFile, scorerPath, workDir string, timeLimitSec, memLimitMB int) (ts tcScore, timeMs int) {
	input, err := os.ReadFile(inFile)
	if err != nil {
		return tcScore{verdict: "IE"}, 0
	}

	// Resource limits applied as best-effort: ulimit -v is unsupported in some container
	// environments (macOS Docker Desktop), so we use ; not && to avoid aborting the run.
	vmKB := (memLimitMB + 256) * 1024
	ulimitPrefix := fmt.Sprintf("ulimit -s 65536 2>/dev/null; ulimit -v %d 2>/dev/null; ulimit -f 131072 2>/dev/null; ", vmKB)
	var cmd *exec.Cmd
	switch lang {
	case "cpp17", "cpp20":
		cmd = exec.Command("/bin/sh", "-c", ulimitPrefix+binPath)
	case "python3":
		cmd = exec.Command("/bin/sh", "-c", ulimitPrefix+"python3 "+binPath)
	default:
		return tcScore{verdict: "IE"}, 0
	}

	// Setpgid puts the shell (and all children) in their own process group.
	// On timeout we kill -pgid to guarantee no orphan processes survive.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var buf bytes.Buffer
	cmd.Stdin = bytes.NewReader(input)
	cmd.Stdout = &limitWriter{w: &buf, rem: 64 * 1024 * 1024} // 64 MB cap

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return tcScore{verdict: "IE"}, 0
	}

	deadline := time.Duration(timeLimitSec)*time.Second + 500*time.Millisecond
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	var runErr error
	timedOut := false
	select {
	case runErr = <-done:
	case <-timer.C:
		timedOut = true
		// Kill the entire process group — catches orphaned children from shell forks.
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			cmd.Process.Kill() // fallback: direct kill in case pgid kill fails
		}
		// Hard deadline on the drain: if the process somehow doesn't die in 5s,
		// move on anyway so the judge doesn't block forever.
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			log.Printf("warning: process did not exit after SIGKILL within 5s")
		}
	}

	elapsed := int(time.Since(start).Milliseconds())

	if timedOut {
		return tcScore{verdict: "TLE"}, elapsed
	}
	if runErr != nil {
		return tcScore{verdict: "RTE"}, elapsed
	}

	if scorerPath == "" {
		// Default diff comparison
		expected, err := os.ReadFile(outFile)
		if err != nil {
			return tcScore{verdict: "IE"}, elapsed
		}
		if normalize(buf.String()) == normalize(string(expected)) {
			return tcScore{verdict: "AC"}, elapsed
		}
		return tcScore{verdict: "WA"}, elapsed
	}

	// Custom scorer: write contestant output to temp file, invoke scorer.
	// Protocol: ./scorer <input> <expected_output> <contestant_output>
	// Scorer writes verdict to stdout (tcframe two-line format).
	contestantOut := filepath.Join(workDir, "contestant.out")
	if err := os.WriteFile(contestantOut, buf.Bytes(), 0644); err != nil {
		return tcScore{verdict: "IE"}, elapsed
	}

	scorerCtx, scorerCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer scorerCancel()

	var scorerStdout bytes.Buffer
	scorerCmd := exec.CommandContext(scorerCtx, scorerPath, inFile, outFile, contestantOut)
	scorerCmd.Stdout = &scorerStdout
	scorerCmd.Run() // verdict is in stdout regardless of exit code

	return parseCheckerVerdict(scorerStdout.String()), elapsed
}

// runCaseInteractive connects the solution and communicator via a named pipe.
// Protocol (from tcframe CommunicatorEvaluator):
//
//	mkfifo pipe
//	./communicator <input> < pipe | ./solution > pipe
//
// The communicator writes its verdict to stderr (tcframe two-line format).
func runCaseInteractive(lang, binPath, inFile, communicatorPath, workDir string, timeLimitSec, memLimitMB int) (ts tcScore, timeMs int) {
	pipePath := filepath.Join(workDir, "comm.pipe")
	commErrPath := filepath.Join(workDir, "comm.stderr")

	vmKB := (memLimitMB + 256) * 1024
	var innerCmd string
	switch lang {
	case "cpp17", "cpp20":
		innerCmd = fmt.Sprintf(`ulimit -s 65536 && ulimit -v %d && "%s"`, vmKB, binPath)
	case "python3":
		innerCmd = fmt.Sprintf(`ulimit -v %d && python3 "%s"`, vmKB, binPath)
	default:
		return tcScore{verdict: "IE"}, 0
	}

	// Write a shell script: both processes run concurrently via a named FIFO.
	// SIGPIPE on the solution side (communicator closed pipe) is normal; the
	// pipeline exit status is unreliable — we use communicator stderr for verdict.
	scriptPath := filepath.Join(workDir, "interactive.sh")
	script := fmt.Sprintf(`#!/bin/sh
PIPE="%s"
rm -f "$PIPE"
mkfifo "$PIPE"
"%s" "%s" < "$PIPE" 2>"%s" | /bin/sh -c '%s' > "$PIPE"
EXIT=$?
rm -f "$PIPE"
exit $EXIT
`, pipePath, communicatorPath, inFile, commErrPath, innerCmd)

	if err := os.WriteFile(scriptPath, []byte(script), 0755); err != nil {
		return tcScore{verdict: "IE"}, 0
	}

	cmd := exec.Command("/bin/sh", scriptPath)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	deadline := time.Duration(timeLimitSec)*time.Second + 500*time.Millisecond
	timer := time.NewTimer(deadline)
	defer timer.Stop()

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return tcScore{verdict: "IE"}, 0
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	timedOut := false
	select {
	case <-done:
	case <-timer.C:
		timedOut = true
		if cmd.Process != nil {
			syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			cmd.Process.Kill()
		}
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			log.Printf("warning: interactive process did not exit after SIGKILL within 5s")
		}
	}

	elapsed := int(time.Since(start).Milliseconds())

	if timedOut {
		return tcScore{verdict: "TLE"}, elapsed
	}

	commErrContent, _ := os.ReadFile(commErrPath)
	if strings.TrimSpace(string(commErrContent)) == "" {
		return tcScore{verdict: "RTE"}, elapsed
	}

	return parseCheckerVerdict(string(commErrContent)), elapsed
}

// limitWriter caps how many bytes are written to the underlying writer.
// Excess bytes are silently dropped — the buffer stays bounded even if
// a solution prints gigabytes before crashing (RTE/TLE).
type limitWriter struct {
	w   *bytes.Buffer
	rem int64
}

func (lw *limitWriter) Write(p []byte) (int, error) {
	if lw.rem <= 0 {
		return len(p), nil
	}
	if int64(len(p)) > lw.rem {
		p = p[:lw.rem]
	}
	n, err := lw.w.Write(p)
	lw.rem -= int64(n)
	return n, err
}

func normalize(s string) string {
	lines := strings.Split(strings.TrimRight(s, "\n\r"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t\r")
	}
	return strings.Join(lines, "\n")
}
