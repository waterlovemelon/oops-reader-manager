// Command backfill-cover-variants generates immutable cover renditions for
// books imported before the three-variant pipeline existed.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"github.com/oops-reader/oops-reader-manager/service/internal/catalog"
	"github.com/oops-reader/oops-reader-manager/service/internal/config"
	platformdb "github.com/oops-reader/oops-reader-manager/service/internal/platform/db"
)

func main() {
	apply := flag.Bool("apply", false, "write renditions and update catalog_books; default is dry run")
	bookKey := flag.String("book-key", "", "backfill only one book key")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}
	pool, err := platformdb.Open(cfg.Database)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	store := catalog.NewMySQLStore(pool)
	service := catalog.NewService(store, catalog.NewLocalStorage(cfg.Catalog.Storage.Root, cfg.Catalog.Storage.TempRoot), []catalog.Importer{catalog.EPUBImporter{}, catalog.TXTImporter{}})
	ctx := context.Background()

	if *bookKey != "" {
		backfillOne(ctx, service, *bookKey, *apply)
		return
	}
	for offset := 0; ; offset += 100 {
		books, total, err := store.List(ctx, "", "", 100, offset)
		if err != nil {
			log.Fatal(err)
		}
		for _, book := range books {
			backfillOne(ctx, service, book.BookKey, *apply)
		}
		if offset+len(books) >= total {
			return
		}
	}
}

func backfillOne(ctx context.Context, service *catalog.Service, bookKey string, apply bool) {
	if !apply {
		fmt.Printf("would backfill %s\n", bookKey)
		return
	}
	changed, err := service.BackfillCoverVariants(ctx, bookKey)
	if err != nil {
		log.Printf("backfill %s: %v", bookKey, err)
		return
	}
	if changed {
		fmt.Printf("backfilled %s\n", bookKey)
	}
}
