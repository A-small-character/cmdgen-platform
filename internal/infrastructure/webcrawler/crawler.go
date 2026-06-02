package webcrawler

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/A-small-character/cmdgen-platform/internal/domain/knowledge"
	"github.com/A-small-character/cmdgen-platform/pkg/config"
)

// DocCrawler 官方文档爬取器
type DocCrawler struct {
	client    *http.Client
	cfg       *config.CrawlerConfig
	sources   map[string]DocSource
}

// DocSource 文档源配置
type DocSource struct {
	Name      string
	BaseURL   string
	Priority  int
	Extractor ContentExtractor
}

// ContentExtractor 内容提取器接口
type ContentExtractor interface {
	Extract(doc *goquery.Document) (title, content string, codeBlocks []string)
}

func NewDocCrawler(cfg *config.CrawlerConfig) *DocCrawler {
	c := &DocCrawler{
		client: &http.Client{
			Timeout: time.Duration(cfg.Timeout) * time.Second,
		},
		cfg:     cfg,
		sources: make(map[string]DocSource),
	}
	c.registerBuiltinSources()
	return c
}

func (c *DocCrawler) registerBuiltinSources() {
	c.sources["elastic_docs"] = DocSource{
		Name:      "Elastic官方文档",
		BaseURL:   "https://www.elastic.co/guide/en/elasticsearch/reference",
		Priority:  1,
		Extractor: &ElasticExtractor{},
	}
	c.sources["cisco_docs"] = DocSource{
		Name:    "Cisco官方文档",
		BaseURL: "https://www.cisco.com/c/en/us/support",
		Priority: 1,
		Extractor: &GenericExtractor{},
	}
	c.sources["huawei_docs"] = DocSource{
		Name:    "华为官方文档",
		BaseURL: "https://support.huawei.com",
		Priority: 1,
		Extractor: &GenericExtractor{},
	}
	c.sources["h3c_docs"] = DocSource{
		Name:    "H3C官方文档",
		BaseURL: "https://www.h3c.com/cn/Support/Resource_Center",
		Priority: 1,
		Extractor: &GenericExtractor{},
	}
	c.sources["linux_man"] = DocSource{
		Name:    "Linux Man Pages",
		BaseURL: "https://man7.org/linux/man-pages",
		Priority: 2,
		Extractor: &ManPageExtractor{},
	}
	c.sources["docker_docs"] = DocSource{
		Name:      "Docker官方文档",
		BaseURL:   "https://docs.docker.com",
		Priority:  1,
		Extractor: &DockerDocsExtractor{},
	}
	c.sources["kubernetes_docs"] = DocSource{
		Name:      "Kubernetes官方文档",
		BaseURL:   "https://kubernetes.io/docs",
		Priority:  1,
		Extractor: &K8sDocsExtractor{},
	}
	c.sources["mysql_docs"] = DocSource{
		Name:      "MySQL官方文档",
		BaseURL:   "https://dev.mysql.com/doc/refman",
		Priority:  1,
		Extractor: &MySQLDocsExtractor{},
	}
}

// SearchResult 搜索结果
type SearchResult struct {
	Title   string   `json:"title"`
	URL     string   `json:"url"`
	Source  string   `json:"source"`
	Snippet string   `json:"snippet"`
	Commands []string `json:"commands"`
}

// Search 在指定源搜索文档
func (c *DocCrawler) Search(ctx context.Context, query, sourceName string) ([]*SearchResult, error) {
	source, ok := c.sources[sourceName]
	if !ok {
		return nil, fmt.Errorf("未知文档源: %s", sourceName)
	}

	// 构建搜索URL（各站点搜索接口不同，这里用通用策略）
	searchURL := buildSearchURL(source.BaseURL, query)

	req, err := http.NewRequestWithContext(ctx, "GET", searchURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	results := extractSearchResults(doc, source)
	return results, nil
}

// FetchAndParse 抓取并解析指定URL
func (c *DocCrawler) FetchAndParse(ctx context.Context, url, sourceName string) (*knowledge.KnowledgeItem, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}

	source := c.sources[sourceName]
	if source.Extractor == nil {
		source.Extractor = &GenericExtractor{}
	}

	title, content, _ := source.Extractor.Extract(doc)
	if title == "" {
		title = doc.Find("title").First().Text()
	}

	return &knowledge.KnowledgeItem{
		Title:     title,
		Content:   content,
		Source:    source.Name,
		SourceURL: url,
		UpdatedAt: time.Now(),
	}, nil
}

// --- 内容提取器实现 ---

type ElasticExtractor struct{}

func (e *ElasticExtractor) Extract(doc *goquery.Document) (title, content string, codeBlocks []string) {
	title = doc.Find("h1.title, h1").First().Text()

	var sb strings.Builder
	doc.Find(".content, article").Each(func(_ int, s *goquery.Selection) {
		sb.WriteString(s.Text())
	})
	content = strings.TrimSpace(sb.String())

	doc.Find("pre code, .pre").Each(func(_ int, s *goquery.Selection) {
		codeBlocks = append(codeBlocks, strings.TrimSpace(s.Text()))
	})
	return
}

type ManPageExtractor struct{}

func (e *ManPageExtractor) Extract(doc *goquery.Document) (title, content string, codeBlocks []string) {
	title = doc.Find("h1").First().Text()

	var sb strings.Builder
	doc.Find("pre, .man-page").Each(func(_ int, s *goquery.Selection) {
		sb.WriteString(s.Text() + "\n")
	})
	content = strings.TrimSpace(sb.String())
	return
}

// DockerDocsExtractor Docker官方文档提取器
type DockerDocsExtractor struct{}

func (e *DockerDocsExtractor) Extract(doc *goquery.Document) (title, content string, codeBlocks []string) {
	title = doc.Find("h1").First().Text()
	var sb strings.Builder
	doc.Find("article, .md-content, main").Each(func(_ int, s *goquery.Selection) {
		sb.WriteString(s.Text())
	})
	content = strings.TrimSpace(sb.String())
	doc.Find("pre code, code.language-bash, code.language-console, code.language-dockerfile").Each(func(_ int, s *goquery.Selection) {
		if t := strings.TrimSpace(s.Text()); len(t) > 5 {
			codeBlocks = append(codeBlocks, t)
		}
	})
	return
}

// K8sDocsExtractor Kubernetes官方文档提取器
type K8sDocsExtractor struct{}

func (e *K8sDocsExtractor) Extract(doc *goquery.Document) (title, content string, codeBlocks []string) {
	title = doc.Find("h1").First().Text()
	var sb strings.Builder
	doc.Find(".td-content, article").Each(func(_ int, s *goquery.Selection) {
		sb.WriteString(s.Text())
	})
	content = strings.TrimSpace(sb.String())
	doc.Find("pre code, code.language-yaml, code.language-bash, code.language-shell").Each(func(_ int, s *goquery.Selection) {
		if t := strings.TrimSpace(s.Text()); len(t) > 5 {
			codeBlocks = append(codeBlocks, t)
		}
	})
	return
}

// MySQLDocsExtractor MySQL官方文档提取器
type MySQLDocsExtractor struct{}

func (e *MySQLDocsExtractor) Extract(doc *goquery.Document) (title, content string, codeBlocks []string) {
	title = doc.Find("h1, .titlepage h1").First().Text()
	var sb strings.Builder
	doc.Find(".section, .chapter, #docs-body").Each(func(_ int, s *goquery.Selection) {
		sb.WriteString(s.Text())
	})
	content = strings.TrimSpace(sb.String())
	doc.Find("pre.programlisting, pre.screen, code.literal").Each(func(_ int, s *goquery.Selection) {
		if t := strings.TrimSpace(s.Text()); len(t) > 5 {
			codeBlocks = append(codeBlocks, t)
		}
	})
	return
}

type GenericExtractor struct{}

func (e *GenericExtractor) Extract(doc *goquery.Document) (title, content string, codeBlocks []string) {
	title = doc.Find("h1, h2").First().Text()

	var sb strings.Builder
	doc.Find("article, main, .content, #content").Each(func(_ int, s *goquery.Selection) {
		sb.WriteString(s.Text())
	})
	if sb.Len() == 0 {
		sb.WriteString(doc.Find("body").Text())
	}
	content = strings.TrimSpace(sb.String())

	doc.Find("pre, code").Each(func(_ int, s *goquery.Selection) {
		if t := strings.TrimSpace(s.Text()); len(t) > 10 {
			codeBlocks = append(codeBlocks, t)
		}
	})
	return
}

func buildSearchURL(baseURL, query string) string {
	if strings.Contains(baseURL, "elastic.co") {
		return fmt.Sprintf("%s/current/search.html?q=%s", baseURL, query)
	}
	return fmt.Sprintf("%s?q=%s", baseURL, query)
}

func extractSearchResults(doc *goquery.Document, source DocSource) []*SearchResult {
	var results []*SearchResult
	doc.Find("a[href]").Each(func(_ int, s *goquery.Selection) {
		href, _ := s.Attr("href")
		text := strings.TrimSpace(s.Text())
		if text != "" && len(text) > 10 && strings.HasPrefix(href, "http") {
			results = append(results, &SearchResult{
				Title:   text,
				URL:     href,
				Source:  source.Name,
				Snippet: text,
			})
		}
		if len(results) >= 10 {
			return
		}
	})
	return results
}
