package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func RunInCgroup(cgroup, output string, pullImageInputStruct Input, command string, args ...string) error {
	if cgroup == "" || output == "" || command == "" {
		return fmt.Errorf("missing required arguments")
	}

	cgroupProcsFile := fmt.Sprintf("/sys/fs/cgroup/%s/cgroup.procs", cgroup)
	outfile, err := os.Create(output)

	var img = pullImageInputStruct.imgUrl + ":" + pullImageInputStruct.imgTag

	cmd := exec.Command("sudo", "ECR_PULL_PARALLEL=6", "./bin/ecr-pull", img)
	cmd.Dir = pullImageInputStruct.pathToBinary

	if err != nil {
		return fmt.Errorf("error creating output file: %v", err)
	}
	defer outfile.Close()

	// Use this to debug potential errors
	// multiWriter := io.MultiWriter(outfile, os.Stdout)
	// cmd.Stdout = multiWriter
	// cmd.Stderr = multiWriter
	cmd.Stdout = outfile
	cmd.Stderr = outfile

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("error starting command: %v", err)
	}

	commandPid := cmd.Process.Pid
	fmt.Printf("writing pid %d to file %s\n", commandPid, cgroupProcsFile)
	if err := addPidToCgroup(cgroupProcsFile, commandPid); err != nil {
		return fmt.Errorf("error adding PID to cgroup: %v", err)
	}

	addChildPids(cgroupProcsFile, commandPid)

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("error waiting for command: %v", err)
	}

	fmt.Println("ECR pull complete")
	return nil
}

func addPidToCgroup(cgroupProcsFile string, pid int) error {
	pidStr := strconv.Itoa(pid)
	cmd := exec.Command("sudo", "tee", "-a", cgroupProcsFile)
	cmd.Stdin = strings.NewReader(pidStr)
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func addChildPids(cgroupProcsFile string, parentPid int) {
	fmt.Println("addChildPids parentPid : ", parentPid)
	childPids, err := getChildPids(parentPid)
	if err != nil {
		fmt.Println("Error getting child PIDs:", err)
		return
	}

	for _, childPid := range childPids {
		if err := addPidToCgroup(cgroupProcsFile, childPid); err != nil {
			fmt.Println("Error adding child PID to cgroup:", err)
		}
		addChildPids(cgroupProcsFile, childPid)
	}
}

func getChildPids(parentPid int) ([]int, error) {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(parentPid)).Output()
	if err != nil {
		return nil, err
	}

	var pids []int
	for _, line := range strings.Split(string(out), "\n") {
		if line != "" {
			pid, err := strconv.Atoi(line)
			if err != nil {
				return nil, err
			}
			pids = append(pids, pid)
		}
	}

	return pids, nil
}
