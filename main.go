package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	pathToBinary    = "../amazon-ecr-containerd-resolver"
	imgName         = "1gb-single-layer"
	repoArn         = "ecr.aws/arn:aws:ecr:us-west-1:020023120753:repository/"
	img             = repoArn + imgName + ":latest"
	resultsFile     = imgName + "/results.csv"
	percentilesFile = imgName + "/percentiles.csv"
)

func main() {
	// Setup
	setup()

	// Main loop
	runMainLoop()

	// Calculate and save percentiles
	calculateAndSavePercentiles()
}

func setup() {
	os.MkdirAll(imgName, 0777)
	os.Chmod(imgName, 0777)
	os.Remove(resultsFile)
	os.RemoveAll("./bin")

	// Build project
	cmd := exec.Command("sudo", "make", "build")
	cmd.Dir = pathToBinary
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("Build failed:", err)
		os.Exit(1)
	}

	// Prepare CSV file
	file, err := os.Create(resultsFile)
	if err != nil {
		fmt.Println("Error creating results file:", err)
		os.Exit(1)
	}
	defer file.Close()
	file.WriteString("Run,ParallelLayers,PullTime,Unpack,Speed\n")
}

func runMainLoop() {
	for i := 1; i <= 4; i++ {
		for j := 1; j <= 10; j++ {
			fmt.Fprintf(os.Stderr, "Run: %d with parallel arg: %d\n", j, i)
			ecrPullParallel := 7 - i

			// Set up environment
			restartContainerd()

			cgroupParent := "ecr-pull-benchmark"
			cgroupChild := fmt.Sprintf("count-%d-parallel-%d-slice", j, ecrPullParallel)
			cgroup := filepath.Join(cgroupParent, cgroupChild)
			setupCgroup(cgroup)

			outputFile := "/tmp/" + cgroupChild

			// Pull image and collect data
			runTestScript(cgroup, outputFile, ecrPullParallel)

			elapsed, unpack, time, unpackTime, speed := extractData(outputFile)
			fmt.Fprintln(os.Stderr, elapsed)
			fmt.Println("unpack", unpack)

			if j > 2 {
				appendToCSV(resultsFile, []string{strconv.Itoa(j), strconv.Itoa(ecrPullParallel), time, unpackTime, speed})
			}

			// Clean up
			os.Remove(outputFile)
			removeCgroup(cgroup)
		}
	}
}

func restartContainerd() {
	exec.Command("sudo", "service", "containerd", "stop").Run()
	exec.Command("sudo", "rm", "-rf", "/var/lib/containerd").Run()
	exec.Command("sudo", "mkdir", "-p", "/var/lib/containerd").Run()
	exec.Command("sudo", "service", "containerd", "start").Run()
}

func setupCgroup(cgroup string) {
	os.MkdirAll("/sys/fs/cgroup/"+cgroup, 0755)
	exec.Command("sudo", "sh", "-c", "echo '+memory' | sudo tee /sys/fs/cgroup/"+filepath.Dir(cgroup)+"/cgroup.subtree_control").Run()
	exec.Command("sudo", "sh", "-c", "echo '+cpu' | sudo tee /sys/fs/cgroup/"+filepath.Dir(cgroup)+"/cgroup.subtree_control").Run()
}

func removeCgroup(cgroup string) {
	os.RemoveAll("/sys/fs/cgroup/" + cgroup)
}

func runTestScript(cgroup, outputFile string, ecrPullParallel int) {
	cmd := exec.Command("sudo", "./test.sh", cgroup, outputFile, "sudo", fmt.Sprintf("ECR_PULL_PARALLEL=%d", ecrPullParallel), "./bin/ecr-pull", img)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("Test script failed:", err)
	}
}

func extractData(outputFile string) (string, string, string, string, string) {
	file, err := os.Open(outputFile)
	if err != nil {
		fmt.Println("Error reading output file:", err)
		return "", "", "", "", ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var elapsed, unpack string
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "elapsed") {
			elapsed = line
		}
		if strings.Contains(line, "unpackTime") {
			unpack = line
		}
	}

	time := extractValue(elapsed, 2)
	unpackTime := extractValue(unpack, 2)
	speed := extractSpeed(elapsed)

	return elapsed, unpack, time, unpackTime, speed
}

func extractValue(line string, fieldIndex int) string {
	fields := strings.Fields(line)
	if len(fields) > fieldIndex {
		return strings.TrimSuffix(fields[fieldIndex], "s")
	}
	return ""
}

func extractSpeed(line string) string {
	fields := strings.Fields(line)
	if len(fields) > 0 {
		speed := strings.Trim(fields[len(fields)-1], "()")
		return speed
	}
	return ""
}

func appendToCSV(filePath string, record []string) {
	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		fmt.Println("Error opening CSV file:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write(record)
	writer.Flush()
}

func calculateAndSavePercentiles() {
	inputFile, err := os.Open(resultsFile)
	if err != nil {
		fmt.Println("Error opening results file:", err)
		return
	}
	defer inputFile.Close()

	reader := csv.NewReader(inputFile)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("Error reading CSV file:", err)
		return
	}

	headers := records[0]
	data := records[1:]

	columnIndices := map[string]int{}
	for i, header := range headers {
		columnIndices[header] = i
	}

	pullTimePercentiles := extractAndCalculatePercentiles(data, columnIndices["PullTime"])
	unpackPercentiles := extractAndCalculatePercentiles(data, columnIndices["Unpack"])
	speedPercentiles := extractAndCalculatePercentiles(data, columnIndices["Speed"])

	savePercentiles(pullTimePercentiles, unpackPercentiles, speedPercentiles)
}

func extractAndCalculatePercentiles(data [][]string, columnIndex int) []float64 {
	var values []float64
	for _, record := range data {
		value, _ := strconv.ParseFloat(record[columnIndex], 64)
		values = append(values, value)
	}

	sort.Float64s(values)
	count := len(values)
	return calculatePercentiles(values, count)
}

func calculatePercentiles(values []float64, count int) []float64 {
	p0 := values[0]
	p50 := values[count/2]
	p90 := values[int(float64(count)*0.9)]
	p100 := values[count-1]
	return []float64{p0, p50, p90, p100}
}

func savePercentiles(pullTimePercentiles, unpackPercentiles, speedPercentiles []float64) {
	file, err := os.Create(percentilesFile)
	if err != nil {
		fmt.Println("Error creating percentiles file:", err)
		return
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	writer.Write([]string{"Metric", "p0", "p50", "p90", "p100"})
	writer.Write(formatPercentiles("Pulltime", pullTimePercentiles))
	writer.Write(formatPercentiles("Unpack", unpackPercentiles))
	writer.Write(formatPercentiles("Speed", speedPercentiles))
	writer.Flush()

	fmt.Println("Percentiles calculated and saved to", percentilesFile)
}

func formatPercentiles(metric string, percentiles []float64) []string {
	return []string{
		metric,
		fmt.Sprintf("%.2f", percentiles[0]),
		fmt.Sprintf("%.2f", percentiles[1]),
		fmt.Sprintf("%.2f", percentiles[2]),
		fmt.Sprintf("%.2f", percentiles[3]),
	}
}
