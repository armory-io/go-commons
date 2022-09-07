package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/fatih/color"
	"io"
	"k8s.io/utils/strings/slices"
	"log"
	"os"
	"os/exec"
	"strings"
)

var red = color.New(color.FgRed)
var boldRed = red.Add(color.Bold)
var green = color.New(color.FgGreen)

type TestOutput struct {
	Time    string  `json:"Time"`
	Action  string  `json:"Action"` // output, skip, run, pass, or fail
	Package string  `json:"Package"`
	Output  string  `json:"Output"`
	Elapsed float64 `json:"Elapsed"`
	Test    string  `json:"Test"`
}

func main() {
	appName := os.Getenv("APP_NAME")
	buildDir := os.Getenv("BUILD_DIR")
	fmt.Println("Fetching packages for test command")
	listCmd := exec.Command("go", "list", "./...")
	var out bytes.Buffer
	listCmd.Stdout = &out
	listCmd.Stderr = os.Stderr
	err := listCmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	args := []string{
		"test",
		fmt.Sprintf("--coverprofile=%s/reports/profile.cov", buildDir),
	}

	packages := slices.Filter(nil, strings.Split(strings.TrimSpace(out.String()), "\n"), func(pkg string) bool {
		return !strings.Contains(pkg, "/e2e")
	})

	args = append(args, packages...)
	args = append(args, "-json")

	cmd := exec.Command("go", args...)

	fmt.Println("Executing the following test command:")
	fmt.Println("------------------------------------------------------------------------------------------------------------------------------------")
	fmt.Println(cmd.String())
	fmt.Println("------------------------------------------------------------------------------------------------------------------------------------")

	cmd.Stderr = os.Stderr

	p, err := cmd.StdoutPipe()
	if err != nil {
		fmt.Printf("Failed to execute command, Err: %s\n", err.Error())
		os.Exit(1)
	}

	done := make(chan bool)
	go streamTestResults(buildDir, p, done)

	err = cmd.Start()
	if err != nil {
		fmt.Printf("Failed to execute command, Err: %s\n", err.Error())
		os.Exit(1)
	}

	<-done
	didTestsPass := true
	if err = cmd.Wait(); err != nil {
		didTestsPass = false
	}

	fmt.Println("Test command complete, generating html report...")
	reportCmd := exec.Command(
		"go-test-report",
		"--title", appName,
		"-v",
		"--output", fmt.Sprintf("%s/reports/test_report.html", buildDir),
	)
	reportCmd.Stderr = os.Stderr
	testResults, err := os.Open(fmt.Sprintf("%s/reports/tests-results.json", buildDir))
	reportCmd.Stdin = testResults
	if err = reportCmd.Run(); err != nil {
		fmt.Printf("Error generating html report, err: %s\n", err.Error())
	}
	fmt.Println("html report generation complete")

	if !didTestsPass {
		fmt.Printf("\nATTENTION: The tests failed, exiting 1 now!\n")
		os.Exit(1)
	}
}

func streamTestResults(buildDir string, r io.ReadCloser, done chan bool) {
	fmt.Println("Streaming tests results")
	fmt.Println("------------------------------------------------------------------------------------------------------------------------------------")

	var failedTests []string
	path := fmt.Sprintf("%s/reports/tests-results.json", buildDir)
	results, err := os.OpenFile(path, os.O_TRUNC|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("failed creating results file: %s", err)
	}
	writer := bufio.NewWriter(results)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		out := &TestOutput{}
		line := scanner.Text()
		if err := json.Unmarshal([]byte(line), out); err != nil {
			continue
		}
		if out.Action == "" {
			continue
		}
		if out.Action == "fail" && out.Test != "" {
			failedTests = append(failedTests, out.Test)
		}
		trimmedOut := strings.TrimSpace(out.Output)
		if strings.HasPrefix(trimmedOut, "---") || strings.HasPrefix(trimmedOut, "===") {
			colorize(out.Output)
		}
		_, _ = writer.WriteString(fmt.Sprintf("%s\n", line))
	}
	writer.Flush()
	results.Close()
	if len(failedTests) > 0 {
		fmt.Println()
		boldRed.Println("------------------------------------------------------------------------------------------------------------------------------------")
		boldRed.Println("                                                    !!!! TEST FAILURES !!!!")
		boldRed.Println("------------------------------------------------------------------------------------------------------------------------------------")
		for i := len(failedTests) - 1; i >= 0; i-- {
			fmt.Printf("- %s\n", failedTests[i])
		}
		fmt.Println("------------------------------------------------------------------------------------------------------------------------------------")
		fmt.Println("The human readable tests report w/ stdout and stderr will be uploaded in the reports artifact to this workflow")
		fmt.Println("------------------------------------------------------------------------------------------------------------------------------------")
	}
	done <- true
}

func colorize(output string) {
	if strings.Contains(output, "--- PASS") {
		green.Print(output)
		return
	}

	if strings.Contains(output, "--- FAIL") {
		boldRed.Print(output)
		return
	}

	fmt.Print(output)
}
