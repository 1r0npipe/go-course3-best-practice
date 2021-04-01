package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	// максимально допустимое число ошибок при парсинге
	errorsLimit = 100000

	// число результатов, которые хотим получить
	resultsLimit = 10000
)

var (
	// адрес в интернете (например, https://en.wikipedia.org/wiki/Lionel_Messi)
	url string

	// насколько глубоко нам надо смотреть (например, 10)
	depthLimit int

	timeout int = 3
)

// Как вы помните, функция инициализации стартует первой
func init() {
	// задаём и парсим флаги
	flag.StringVar(&url, "url", "", "url address")
	flag.IntVar(&depthLimit, "depth", 3, "max depth for run")
	flag.Parse()

	// Проверяем обязательное условие
	if url == "" {
		log.Print("no url set by flag")
		flag.PrintDefaults()
		log.Fatal("No url provided or incorrect format")
	}
}

func main() {
	fmt.Println("PID:", os.Getpid())
	started := time.Now()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	
	crawler := newCrawler(depthLimit)

	// запускаем горутину с каналом смотрящим за сигналами
	go watchSignals(crawler, cancel)

	// создаём канал для результатов
	results := make(chan crawlResult)

	// запускаем горутину для чтения из каналов
	done := watchCrawler(ctx, results, errorsLimit, resultsLimit)

	// запуск основной логики
	// внутри есть рекурсивные запуски анализа в других горутинах
	crawler.run(ctx, url, results, 0)

	// ждём завершения работы чтения в своей горутине
	<-done

	log.Println(time.Since(started))
}

// ловим сигналы выключения
func watchSignals(c *crawler, cancel context.CancelFunc) {
	osSignalChan := make(chan os.Signal, 1)

	signal.Notify(osSignalChan,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGUSR1)
	
	// ожидаем сигнал на повышение глубины поиска либо по умолчанию на вероятное завершение
	// если придет сигнал отличный от SIGUSR1
	for {
		sig := <-osSignalChan
		switch sig {
		case syscall.SIGUSR1: 
			c.maxDepth += 10
			log.Printf("got signal %s, the max depth gets increased by 10, and now is: %d", sig.String(), c.maxDepth)
			continue
		default:
			log.Printf("got signal %q", sig.String())
			cancel()
		}
	}
}

func watchCrawler(ctx context.Context, results <-chan crawlResult, maxErrors, maxResults int) chan struct{} {
	readersDone := make(chan struct{})

	go func() {
		defer close(readersDone)
		for {
			select {
			case <-ctx.Done():
				return

			case result := <-results:
				if result.err != nil {
					maxErrors--
					if maxErrors <= 0 {
						log.Println("max errors exceeded")
						return
					}
					continue
				}

				log.Printf("crawling result: %v", result.msg)
				maxResults--
				if maxResults <= 0 {
					log.Println("got max results")
					return
				}
			}
		}
	}()

	return readersDone
}