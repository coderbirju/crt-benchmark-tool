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

// var (
// 	pathToBinary string
// 	imgName      string
// 	repoArn      string
// )

// // func init() {
// // 	flag.StringVar(&pathToBinary, "path", "/home/ec2-user/amazon-ecr-containerd-resolver", "Absolute path to the binary to be tested")
// // 	flag.StringVar(&imgName, "img-name", "1gb-single-layer", "Image name")
// // 	flag.StringVar(&repoArn, "arn", "ecr.aws/arn:aws:ecr:us-west-1:020023120753:repository/", "Repository ARN")
// }

const (
	pathToBinary    = "/home/ec2-user/amazon-ecr-containerd-resolver"
	imgName         = "1gb-single-layer"
	repoArn         = "ecr.aws/arn:aws:ecr:us-west-1:020023120753:repository/"
	img             = repoArn + imgName + ":latest"
	resultsFile     = imgName + "/results.csv"
	percentilesFile = imgName + "/percentiles.csv"
)

func main() {

	// flag.Parse()

	// img := repoArn + imgName + ":latest"
	// resultsFile     := imgName + "/results.csv"
	setup()

	runMainLoop()
}

func setup() {
	os.MkdirAll(imgName, 0777)
	os.Chmod(imgName, 0777)
	os.Remove(pathToBinary + imgName + "/results.csv")
	os.RemoveAll(pathToBinary + "/bin")

	cmd := exec.Command("make", "build")
	cmd.Dir = pathToBinary
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("Build failed:", err)
		os.Exit(1)
	}

	file, err := os.Create(imgName + "/results.csv")
	if err != nil {
		fmt.Println("Error creating results file:", err)
		os.Exit(1)
	}
	defer file.Close()
	file.WriteString("Run,ParallelLayers,PullTime,Unpack,Speed\n")
}

func runMainLoop() {
	for i := 1; i <= 4; i++ {
		for j := 1; j <= 1; j++ {
			fmt.Fprintf(os.Stderr, "Run: %d with parallel arg: %d\n", j, i)
			ecrPullParallel := 7 - i

			restartContainerd()

			cgroupParent := "crt-benchmark"
			cgroupChild := fmt.Sprintf("count-%d-parallel-%d-slice", j, ecrPullParallel)
			cgroup := filepath.Join(cgroupParent, cgroupChild)
			setupCgroup(cgroupParent, cgroupChild)

			outputFile := "/tmp/" + cgroupChild

			runTestScript(cgroup, outputFile, ecrPullParallel)

			elapsed, unpack, time, unpackTime, speed := extractData(outputFile)
			fmt.Fprintln(os.Stderr, elapsed)
			fmt.Println("unpack", unpack)
			// fmt.Println("elapsed, unpack, time, unpackTime, speed", elapsed, unpack, time, unpackTime, speed)

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

func setupCgroup(cgroupParent string, cgroupChild string) {
	fmt.Println("------------------------ setupCgroup------------------------")
	cgroup := filepath.Join(cgroupParent, cgroupChild)
	// os.MkdirAll("/sys/fs/cgroup/"+cgroup, 0755)
	exec.Command("sudo", "mkdir", "-p", "/sys/fs/cgroup/"+cgroup).Run()
	if stat, err := os.Stat("/sys/fs/cgroup/" + cgroup); err == nil && stat.IsDir() {
		fmt.Println("path /sys/fs/cgroup/" + cgroup + " is a directory")
	} else {
		fmt.Println("path is not a directory", err)
	}
	exec.Command("sudo", "sh", "-c", "echo '+memory' | sudo tee /sys/fs/cgroup/"+cgroupParent+"/cgroup.subtree_control").Run()
	exec.Command("sudo", "sh", "-c", "echo '+cpu' | sudo tee /sys/fs/cgroup/"+cgroupParent+"/cgroup.subtree_control").Run()
}

func removeCgroup(cgroup string) {
	os.RemoveAll("/sys/fs/cgroup/" + cgroup)
}

func runTestScript(cgroup, outputFile string, ecrPullParallel int) {

	// cmd := exec.Command("ls")
	cmd := exec.Command("sudo", "./test.sh", cgroup, outputFile, "sudo", "ECR_PULL_PARALLEL=6", "./bin/ecr-pull", img)
	// cmd := exec.Command("sudo", "./test.sh", cgroup, outputFile, "sleep 1000")
	cmd.Dir = pathToBinary
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
		// fmt.Println("line: ", line)
		if strings.Contains(line, "elapsed") {
			elapsed = line
			// fmt.Println("elapsed: ", elapsed)
		}
		if strings.Contains(line, "unpackTime") {
			unpack = line
			// fmt.Println("unpack: ", unpack)
		}
	}

	time := extractValue(elapsed, 1)
	unpackTime := extractValue(unpack, 1)
	speed := extractSpeed(elapsed)

	fmt.Println("Extracted data at 169 : elapsed, unpack, time, unpackTime, speed ", elapsed, unpack, time, unpackTime, speed)

	return elapsed, unpack, time, unpackTime, speed
}

func extractValue(line string, fieldIndex int) string {
	fields := strings.Fields(line)
	// fmt.Println("len(fields) for line", line, " = ", len(fields))
	if len(fields) > fieldIndex {
		// fmt.Println("fields[fieldIndex(1)]", fields[fieldIndex])
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
