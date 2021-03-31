package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/net/html"
)

// парсим страницу
func parse(ctx context.Context, url string) (*html.Node, error) {
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("the timeout for parser is caught up, url: %v", url)
	default:	 	
		// что здесь должно быть вместо http.Get? :)
		// судя по тому что я прочитал, лучше использовать Transport or Clinet:
		// https://golang.org/pkg/net/http/
		// Clients and Transports are safe for concurrent use by multiple goroutines and for efficiency should only be created once and re-used.
		// r, err := http.Get(url)
		client := &http.Client{}
		r, err := client.Get(url)
		if err != nil {
			return nil, fmt.Errorf("can't get page")
		}
		b, err := html.Parse(r.Body)
		if err != nil {
			return nil, fmt.Errorf("can't parse page")
		}
		return b, err
	}
}

// ищем заголовок на странице
func pageTitle(ctx context.Context, n *html.Node) string {
	select {
	case <-ctx.Done():
		log.Printf("the timeout for title is caught up")
		return ""
	default:
		var title string
		if n.Type == html.ElementNode && n.Data == "title" {
			return n.FirstChild.Data
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			title = pageTitle(ctx, c)
			if title != "" {
				break
			}
		}
		return title
	}
}

// ищем все ссылки на страницы. Используем мапку чтобы избежать дубликатов
func pageLinks(ctx context.Context, links map[string]struct{}, n *html.Node) map[string]struct{} {
	select {
	case <-ctx.Done():
		log.Printf("the timeout for link is caught up")
		return nil
	default:
		if links == nil {
			links = make(map[string]struct{})
		}

		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key != "href" {
					continue
				}

				// костылик для простоты
				if _, ok := links[a.Val]; !ok && len(a.Val) > 2 && a.Val[:2] == "//" {
					links["http://"+a.Val[2:]] = struct{}{}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			links = pageLinks(ctx, links, c)
		}
		return links
	}
}