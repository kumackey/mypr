package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"time"

	"github.com/google/go-github/v57/github"
	"golang.org/x/oauth2"
)

func getToken() (string, error) {
	// 1. 環境変数 GITHUB_TOKEN を優先
	if token := os.Getenv("GITHUB_TOKEN"); token != "" {
		return token, nil
	}

	// 2. gh auth token コマンドで取得
	cmd := exec.Command("gh", "auth", "token")
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("GITHUB_TOKEN not set and gh CLI not available: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

func main() {
	token, err := getToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		fmt.Fprintln(os.Stderr, "Please set GITHUB_TOKEN or run 'gh auth login'")
		os.Exit(1)
	}

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(ctx, ts)
	client := github.NewClient(tc)

	// JST タイムゾーン
	jst := time.FixedZone("JST", 9*60*60)

	// 過去30日間の日付を計算
	now := time.Now().In(jst)
	since := now.AddDate(0, 0, -30).Format("2006-01-02")

	// PR を日付ごとにカウントするマップ
	prCountByDate := make(map[string]int)
	var openCount, mergedCount int

	// Open PR を検索
	openQuery := fmt.Sprintf("is:pr author:@me state:open created:>=%s", since)
	openPRs, err := searchPRs(ctx, client, openQuery)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching open PRs: %v\n", err)
		os.Exit(1)
	}

	for _, pr := range openPRs {
		date := pr.CreatedAt.Time.In(jst).Format("2006-01-02")
		prCountByDate[date]++
		openCount++
	}

	// Merged PR を検索
	mergedQuery := fmt.Sprintf("is:pr author:@me is:merged created:>=%s", since)
	mergedPRs, err := searchPRs(ctx, client, mergedQuery)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error searching merged PRs: %v\n", err)
		os.Exit(1)
	}

	for _, pr := range mergedPRs {
		date := pr.CreatedAt.Time.In(jst).Format("2006-01-02")
		prCountByDate[date]++
		mergedCount++
	}

	// 結果を出力
	fmt.Println("GitHub PR Summary (Last 30 days)")
	fmt.Println()

	// 日付でソートして出力
	var dates []string
	for date := range prCountByDate {
		dates = append(dates, date)
	}
	sort.Strings(dates)

	for _, date := range dates {
		count := prCountByDate[date]
		suffix := "PRs"
		if count == 1 {
			suffix = "PR"
		}
		fmt.Printf("%s: %d %s\n", date, count, suffix)
	}

	total := openCount + mergedCount
	workingDays := len(dates)
	avg := float64(total) / float64(workingDays)

	fmt.Println("---")
	fmt.Printf("Total: %d PRs (Open: %d, Merged: %d)\n", total, openCount, mergedCount)
	fmt.Printf("Working days: %d, Average: %.1f PRs/day\n", workingDays, avg)
}

func searchPRs(ctx context.Context, client *github.Client, query string) ([]*github.Issue, error) {
	var allPRs []*github.Issue

	opts := &github.SearchOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		result, resp, err := client.Search.Issues(ctx, query, opts)
		if err != nil {
			return nil, err
		}

		allPRs = append(allPRs, result.Issues...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	return allPRs, nil
}
