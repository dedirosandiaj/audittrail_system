package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/auditrail/auditrail/internal/application"
	"github.com/auditrail/auditrail/internal/config"
	pgstore "github.com/auditrail/auditrail/internal/storage/postgres"
)

func main() {
	app := &cli.App{
		Name:  "auditrail",
		Usage: "Auditrail admin CLI",
		Commands: []*cli.Command{
			{
				Name:  "apps",
				Usage: "Manage applications (tenants)",
				Subcommands: []*cli.Command{
					{
						Name:  "create",
						Usage: "Register a new application",
						Flags: []cli.Flag{
							&cli.StringFlag{Name: "code", Required: true},
							&cli.StringFlag{Name: "name"},
							&cli.StringFlag{Name: "type", Value: "web"},
						},
						Action: appsCreate,
					},
					{Name: "list", Action: appsList},
					{
						Name:   "rotate-key",
						Flags:  []cli.Flag{&cli.StringFlag{Name: "id", Required: true}},
						Action: appsRotate,
					},
				},
			},
			{Name: "migrate", Usage: "Apply Postgres migrations", Action: migrateCmd},
		},
	}
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loadRepo(ctx context.Context) (*application.Repository, func(), error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, nil, err
	}
	pool, err := pgstore.New(ctx, cfg.Postgres.URL, cfg.Postgres.MaxConns, cfg.Postgres.MaxConnLifetime)
	if err != nil {
		return nil, nil, err
	}
	return application.NewRepository(pool), func() { pool.Close() }, nil
}

func appsCreate(c *cli.Context) error {
	ctx := context.Background()
	repo, closer, err := loadRepo(ctx)
	if err != nil {
		return err
	}
	defer closer()
	app, err := repo.Create(ctx, c.String("code"), c.String("name"), c.String("type"))
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(app, "", "  ")
	fmt.Println(string(out))
	fmt.Fprintln(os.Stderr, "\n⚠️  Save the secret_key now — it will not be shown again.")
	return nil
}

func appsList(c *cli.Context) error {
	ctx := context.Background()
	repo, closer, err := loadRepo(ctx)
	if err != nil {
		return err
	}
	defer closer()
	apps, err := repo.List(ctx)
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(apps, "", "  ")
	fmt.Println(string(out))
	return nil
}

func appsRotate(c *cli.Context) error {
	ctx := context.Background()
	repo, closer, err := loadRepo(ctx)
	if err != nil {
		return err
	}
	defer closer()
	app, err := repo.RotateKey(ctx, c.String("id"))
	if err != nil {
		return err
	}
	out, _ := json.MarshalIndent(app, "", "  ")
	fmt.Println(string(out))
	return nil
}

func migrateCmd(c *cli.Context) error {
	ctx := context.Background()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := pgstore.New(ctx, cfg.Postgres.URL, cfg.Postgres.MaxConns, cfg.Postgres.MaxConnLifetime)
	if err != nil {
		return err
	}
	defer pool.Close()
	return pgstore.Migrate(ctx, pool, "migrations/postgres")
}
