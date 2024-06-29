package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

type Input struct {
	pathToBinary    string
	imgUrl          string
	imgTag          string
	imgName         string
	binaryUrl       string
	binaryArgString string
}

var (
	percentilesFile = "/percentiles.csv"
)

func main() {
	var pullImageInputStruct = validateJsonInput("config.json")

	var resultsFile = pullImageInputStruct.imgName + "/results.csv"
	var statsFile = pullImageInputStruct.imgName + "/benchmarkStats.csv"

	setup(pullImageInputStruct, resultsFile, statsFile)

	runMainLoop(pullImageInputStruct, resultsFile, statsFile)
}

func runMainLoop(pullImageInputStruct Input, resultsFile string, statsFile string) {
	for i := 1; i <= 4; i++ {
		for j := 1; j <= 1; j++ {
			fmt.Fprintf(os.Stderr, "Run: %d with parallel arg: %d\n", j, i)
			ecrPullParallel := 7 - i

			restartContainerd()

			cgroupParent := "crt-benchmark"
			cgroupChild := fmt.Sprintf("count-%d-paralleli-%d-slice", j, ecrPullParallel)
			cgroup := filepath.Join(cgroupParent, cgroupChild)
			setupCgroup(cgroupParent, cgroupChild)

			outputFile := "/tmp/" + cgroupChild

			runTestScript(cgroup, outputFile, pullImageInputStruct)

			memoryPeakPath := filepath.Join("/sys/fs/cgroup/", cgroup, "memory.peak")
			memoryPeakBytes, err := os.ReadFile(memoryPeakPath)
			if err != nil {
				fmt.Printf("Error reading memory.peak: %v\n", err)
				return
			}
			memoryPeakStr := strings.TrimSpace(string(memoryPeakBytes))
			memoryPeak, err := strconv.ParseInt(memoryPeakStr, 10, 64)
			if err != nil {
				fmt.Println("Error strconv memoryPeakStr", err)
			}
			memoryPeakMB := float64(memoryPeak) / (1024 * 1024)
			// fmt.Println("Memory PEAK : ", memoryPeakMB, "MB")

			cpuStatPath := filepath.Join("/sys/fs/cgroup/", cgroup, "cpu.stat")
			cpuStatBytes, err := os.ReadFile(cpuStatPath)
			if err != nil {
				fmt.Printf("Error reading cpu.stat: %v\n", err)
				return
			}

			cpuStatString := string(cpuStatBytes)
			cpuStats := strings.Fields(cpuStatString)

			// for i := 1; i < len(cpuStats); i += 2 {
			// 	fmt.Println(cpuStats[i])
			// }

			// fmt.Println("cpuStat : ", cpuStatString)

			_, _, time, unpackTime, speed := extractData(outputFile)
			// fmt.Fprintln(os.Stderr, elapsed)
			// fmt.Println("unpack", unpack)

			appendToCSV(statsFile, []string{strconv.Itoa(j), strconv.FormatFloat(memoryPeakMB, 'f', 2, 64), cpuStats[1], cpuStats[3], cpuStats[5], cpuStats[7], cpuStats[9], cpuStats[11], cpuStats[13], cpuStats[15], cpuStats[17]})

			if j > 2 {
				appendToCSV(resultsFile, []string{strconv.Itoa(j), strconv.Itoa(ecrPullParallel), time, unpackTime, speed})
			}

			// Clean up
			err = os.Remove(outputFile)
			if err != nil {
				fmt.Println("Error removing outputFile", err)
			}
			removeCgroup(cgroup)
		}
	}
}

func setupCgroup(cgroupParent string, cgroupChild string) {
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

func runTestScript(cgroup, outputFile string, pullImageInputStruct Input) {

	// cmd := exec.Command("ls")
	// cmd := exec.Command("sudo", "./test.sh", cgroup, outputFile, "sudo", "ECR_PULL_PARALLEL=6", "./bin/ecr-pull", img)
	img := pullImageInputStruct.imgUrl + pullImageInputStruct.imgTag
	// binaryPath := pullImageInputStruct.pathToBinary + "./bin/ecr-pull"
	binaryUrl := pullImageInputStruct.binaryUrl
	RunInCgroup(cgroup, outputFile, pullImageInputStruct, "sudo", "ECR_PULL_PARALLEL=6", binaryUrl, img)
	// cmd := exec.Command("sudo", "./test.sh", cgroup, outputFile, "sleep 1000")
	// cmd.Dir = pathToBinary
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// if err := cmd.Run(); err != nil {
	// 	fmt.Println("Test script failed:", err)
	// }
}
