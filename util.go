package main

import (
	"fmt"
	"os"
	"os/exec"
)

func setup(pullImageInputStruct Input, resultsFile string, statsFile string) {
	os.MkdirAll(pullImageInputStruct.imgName, 0777)
	os.Chmod(pullImageInputStruct.imgName, 0777)
	os.Remove(pullImageInputStruct.pathToBinary + pullImageInputStruct.imgName + "/results.csv")
	os.RemoveAll(pullImageInputStruct.pathToBinary + "/bin")

	cmd := exec.Command("make", "build")
	cmd.Dir = pullImageInputStruct.pathToBinary
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Println("Build failed:", err)
		os.Exit(1)
	}

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
