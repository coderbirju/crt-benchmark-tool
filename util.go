package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
)

func setup(pullImageInputStruct Input, resultsFile string, statsFile string) {
	os.MkdirAll(pullImageInputStruct.imgName, 0777)
	os.Chmod(pullImageInputStruct.imgName, 0777)
	os.Remove(pullImageInputStruct.pathToBinary + pullImageInputStruct.imgName + "/results.csv")
	// os.RemoveAll(pullImageInputStruct.pathToBinary + "/bin")

	// Provide the built binaries instead of building a binary again
	// cmd := exec.Command("make", "build")
	// cmd.Dir = pullImageInputStruct.pathToBinary
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr
	// if err := cmd.Run(); err != nil {
	// 	fmt.Println("Build failed:", err)
	// 	os.Exit(1)
	// }

	file, err := os.Create(resultsFile)
	if err != nil {
		fmt.Println("Error creating results file:", err)
		os.Exit(1)
	}
	defer file.Close()

	File, err := os.Create(statsFile)
	if err != nil {
		fmt.Println("Error creating statsFile: ", err)
		os.Exit(1)
	}
	defer File.Close()
	File.WriteString("Run,Memory,usage_usec,user_usec,system_usec,core_sched.force_idle_usec,nr_periods,nr_throttled,throttled_usec,nr_bursts,burst_usec\n")
	// file.WriteString("Run,ParallelLayers,PullTime,Unpack,Speed\n")
}

func restartContainerd() {
	exec.Command("sudo", "service", "containerd", "stop").Run()
	exec.Command("sudo", "rm", "-rf", "/var/lib/containerd").Run()
	exec.Command("sudo", "mkdir", "-p", "/var/lib/containerd").Run()
	exec.Command("sudo", "service", "containerd", "start").Run()
}

func validateJsonInput(fileLocation string) Input {
	var pullImageInputStruct Input
	jsonFile, err := os.Open(fileLocation)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("Config file found, unmarshalling...")
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
	var input map[string]interface{}

	json.Unmarshal([]byte(byteValue), &input)
	if value, ok := input["BinaryPath"].(string); ok {
		pullImageInputStruct.pathToBinary = value
	} else {
		fmt.Println("value provided in BinaryPath variable is not valid")
	}

	if value, ok := input["ImgUrl"].(string); ok {
		pullImageInputStruct.imgUrl = value
	} else {
		fmt.Println("value provided in ImgUrl variable is not valid")
	}

	if value, ok := input["ImgTag"].(string); ok {
		pullImageInputStruct.imgTag = value
	} else {
		fmt.Println("value provided in ImgTag variable is not valid")
	}

	if value, ok := input["ImgName"].(string); ok {
		pullImageInputStruct.imgName = value
	} else {
		fmt.Println("value provided in ImgName variable is not valid")
	}

	if value, ok := input["BinaryUrl"].(string); ok {
		pullImageInputStruct.binaryUrl = value
	} else {
		fmt.Println("value provided in BinaryUrl variable is not valid")
	}

	if value, ok := input["BinaryArgs"].(string); ok {
		pullImageInputStruct.binaryArgString = value
	} else {
		fmt.Println("value provided in binaryArgString variable is not valid")
	}

	return pullImageInputStruct
}

func removeCgroup(cgroup string) {
	os.RemoveAll("/sys/fs/cgroup/" + cgroup)
}
