package main

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/seancfoley/ipaddress-go/ipaddr"
)

// 進捗表示用のカスタムライター
type ProgressWriter struct {
	Total      int64
	Downloaded int64
}

func (pw *ProgressWriter) Write(p []byte) (int, error) {
	n := len(p)
	pw.Downloaded += int64(n)
	if pw.Total > 0 {
		percent := float64(pw.Downloaded) / float64(pw.Total) * 100
		fmt.Printf("\rDownloading... %.2f%% (%d/%d bytes)", percent, pw.Downloaded, pw.Total)
	} else {
		fmt.Printf("\rDownloading... %d bytes", pw.Downloaded)
	}
	return n, nil
}

func main() {
	if len(os.Args) > 1 {
		checkIP(os.Args[1])
		return
	}

	// 1. キャッシュファイル名の設定 (日付 20260315 等を付与)
	today := time.Now().Format("20060102")
	cacheFile := fmt.Sprintf("./output/nro-delegated-stats-%s.txt", today)
	url := "https://ftp.ripe.net/pub/stats/ripencc/nro-stats/latest/nro-delegated-stats"

	// 2. キャッシュの確認
	if _, err := os.Stat(cacheFile); os.IsNotExist(err) {
		log.Printf("Cache not found. Downloading to %s...", cacheFile)
		if err := downloadFileWithProgress(url, cacheFile); err != nil {
			log.Fatal("Download failed: ", err)
		}
		fmt.Println("\nDownload complete.")
	} else {
		log.Printf("Using existing cache: %s", cacheFile)
	}

	// ファイルの解析
	parseAndProcess(cacheFile)
}

func downloadFileWithProgress(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	// プログレス表示の準備
	pw := &ProgressWriter{
		Total: resp.ContentLength,
	}

	// Bodyから読み取ると同時に pw (進捗表示) と out (ファイル) に流し込む
	_, err = io.Copy(out, io.TeeReader(resp.Body, pw))
	return err
}

func parseAndProcess(filename string) {
	file, err := os.Open(filename)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	var v4Strings, v6Strings []string
	var v4nets, v6nets []*ipaddr.IPAddress

	targetCountry := "JP" // 必要に応じて変更可能
	f_4, err := os.Create("./output/IPv4.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f_4.Close()
	f_6, err := os.Create("./output/IPv6.txt")
	if err != nil {
		log.Fatal(err)
	}
	defer f_6.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		fields := strings.Split(line, "|")
		if len(fields) < 5 {
			continue
		}

		registry := fields[0]
		country := fields[1]
		resType := fields[2]
		startIP := fields[3]
		value := fields[4]

		var cidr string
		switch resType {
		case "ipv4":
			count, _ := strconv.ParseFloat(value, 64)
			prefix := 32 - int(math.Log2(count))
			cidr = fmt.Sprintf("%s/%d", startIP, prefix)
			if country == targetCountry {
				fmt.Fprintf(f_4, "%s,%s,%s\n", registry, country, cidr)
			}
		case "ipv6":
			cidr = fmt.Sprintf("%s/%s", startIP, value)
			if country == targetCountry {
				fmt.Fprintf(f_6, "%s,%s,%s\n", registry, country, cidr)
			}
		default:
			continue
		}

		addr := ipaddr.NewIPAddressString(cidr).GetAddress()
		if addr == nil {
			continue
		}

		if addr.IsIPv4() {
			v4Strings = append(v4Strings, cidr)
			v4nets = append(v4nets, addr.ToPrefixBlock())
		} else {
			v6Strings = append(v6Strings, cidr)
			v6nets = append(v6nets, addr.ToPrefixBlock())
		}
	}

	log.Println("Merging and writing results...")
	v4merged, v6merged := ipaddr.MergeToPrefixBlocks(append(v4nets, v6nets...)...)

	saveToFile("./output/IPv4_ISP_merged.txt", v4merged)
	saveAsRTXCommaFormat("./output/RTX_IPv4_filter.txt", v4merged, 410000)
	saveToFile("./output/IPv6_ISP_merged.txt", v6merged)

	fmt.Printf("IPv4: %d -> %d\n", len(v4Strings), len(v4merged))
	fmt.Printf("IPv6: %d -> %d\n", len(v6Strings), len(v6merged))
}

func saveToFile(filename string, nets []*ipaddr.IPAddress) {
	var out []string
	for _, n := range nets {
		out = append(out, n.String())
	}
	os.WriteFile(filename, []byte(strings.Join(out, "\n")), 0644)
}

func checkIP(input string) {
	target := ipaddr.NewIPAddressString(input).GetAddress()
	if target == nil {
		log.Println("invalid IP or CIDR")
		return
	}

	files := []string{"./output/IPv4.txt", "./output/IPv4_ISP_merged.txt", "./output/IPv6.txt", "./output/IPv6_ISP_merged.txt"}
	found := false
	for _, file := range files {
		f, err := os.Open(file)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		line := 1
		for scanner.Scan() {
			cidr := strings.TrimSpace(scanner.Text())

			parts := strings.Split(cidr, ",")
			cidrToMatch := parts[len(parts)-1]

			net := ipaddr.NewIPAddressString(cidrToMatch).GetAddress()
			if net != nil && (net.Contains(target) || target.Contains(net)) {
				fmt.Printf("%s/L%d: %s\n", file, line, cidr)
				found = true
			}
			line++
		}
		f.Close()
	}
	if !found {
		fmt.Println("No match found")
	}
}

func saveAsRTXCommaFormat(filename string, nets []*ipaddr.IPAddress, startFilterNum int) {
	const perLine = 50 // 1行あたりのIP数（安全圏）
	f, _ := os.Create(filename)
	defer f.Close()

	for i := 0; i < len(nets); i += perLine {
		end := i + perLine
		if end > len(nets) {
			end = len(nets)
		}

		// カンマ区切りのリストを作成
		var ipList []string
		for _, n := range nets[i:end] {
			ipList = append(ipList, n.String())
		}

		fmt.Fprintf(f, "ip filter %d pass %s <dest-ip> <protocol> * <dest-port>\n", startFilterNum+(i/perLine), strings.Join(ipList, ","))
	}
}
