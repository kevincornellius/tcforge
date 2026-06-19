package main

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
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

	finalVerdict, score, maxTimeMs, verdicts := judge(sub, contestDir)

	db.Exec("UPDATE submissions SET status='done', verdict=?, score=?, time_ms=? WHERE id=?",
		finalVerdict, score, maxTimeMs, sub.ID)

	for _, v := range verdicts {
		db.Exec("INSERT INTO verdicts (submission_id, test_case, verdict, time_ms, memory_kb) VALUES (?,?,?,?,?)",
			sub.ID, v.testCase, v.verdict, v.timeMs, v.memKb)
	}

	log.Printf("submission %d: %s score=%d time=%dms", sub.ID, finalVerdict, score, maxTimeMs)
	return nil
}

type tcVerdict struct {
	testCase string
	verdict  string
	timeMs   int
	memKb    int
}

func judge(sub submission, contestDir string) (finalVerdict string, score, maxTimeMs int, results []tcVerdict) {
	tcDir := filepath.Join(contestDir, sub.ProblemPath, "tc")

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

	finalVerdict = "AC"
	passed := 0

	for _, inFile := range inFiles {
		base := strings.TrimSuffix(inFile, ".in")
		tcName := filepath.Base(base)

		v, tMs := runCase(sub.Language, binPath, inFile, base+".out", sub.TimeLimit)
		if tMs > maxTimeMs {
			maxTimeMs = tMs
		}
		results = append(results, tcVerdict{testCase: tcName, verdict: v, timeMs: tMs})

		if v == "AC" {
			passed++
		} else if finalVerdict == "AC" {
			finalVerdict = v
		}
	}

	if len(inFiles) > 0 {
		score = passed * 100 / len(inFiles)
	}
	return
}

func compile(lang, code, dir string) (string, error) {
	switch lang {
	case "cpp17":
		src := filepath.Join(dir, "solution.cpp")
		bin := filepath.Join(dir, "solution")
		if err := os.WriteFile(src, []byte(code), 0644); err != nil {
			return "", err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "g++", "-O2", "-std=c++17", "-o", bin, src).CombinedOutput()
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
	case "cpp17":
		cmd = exec.CommandContext(ctx, binPath)
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
