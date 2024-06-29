package main

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

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

func calculateAndSavePercentiles(resultsFile string) {
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
