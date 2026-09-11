package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"lunar-tear/server/internal/database"
	"lunar-tear/server/internal/gametime"
	"lunar-tear/server/internal/runtime"
	"lunar-tear/server/internal/store"
	"lunar-tear/server/internal/store/sqlite"
)

const masterDataPath = "assets/release/20240404193219.bin.e"

// pinnedUserEnv is the environment-variable equivalent of --user, for
// deployments that only configure the server through the environment.
const pinnedUserEnv = "LUNAR_USER"

func main() {
	listen := flag.String("listen", "0.0.0.0:443", "gRPC listen address (host:port)")
	publicAddr := flag.String("public-addr", "127.0.0.1:443", "externally-reachable host:port advertised to clients")
	dbPath := flag.String("db", "db/game.db", "SQLite database path")
	octoURL := flag.String("octo-url", "", "Octo CDN base URL the client will use for assets (e.g. http://10.0.2.2:8080)")
	authURL := flag.String("auth-url", "", "Auth server base URL for Facebook token validation (e.g. http://localhost:3000)")
	adminListen := flag.String("admin-listen", "127.0.0.1:8082", "admin webhook listen address (host:port). Loopback by default; only binds when LUNAR_ADMIN_TOKEN is set.")
	noRegister := flag.Bool("no-register", false, "Disallow new account registrations for clients, when present. Default = false")
	user := flag.String("user", "", "In-game player name to pin every client session to. All logins resolve to this one account regardless of client uuid or stored sessions. Falls back to the "+pinnedUserEnv+" environment variable.")
	flag.Parse()

	if *octoURL == "" {
		log.Fatalf("--octo-url is required (e.g. http://10.0.2.2:8080)")
	}

	db, err := database.Open(*dbPath)
	if err != nil {
		log.Fatalf("open database: %v", err)
	}
	defer db.Close()
	log.Printf("database opened: %s", *dbPath)

	sqliteStore := sqlite.New(db, gametime.Now)

	// Resolved before the master data load so a bad pinned name is reported at once.
	var userStore store.Repository = sqliteStore
	if name, origin := pinnedUser(*user); name != "" {
		userStore = store.NewPinned(sqliteStore, resolvePinnedUser(sqliteStore, name, origin))
	}

	holder, err := runtime.NewHolder(masterDataPath)
	if err != nil {
		log.Fatalf("init master data: %v", err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	grpcServer := startGRPC(*listen, *publicAddr, *octoURL, *authURL, userStore, holder, *noRegister)

	startAdmin(*adminListen, holder)

	<-ctx.Done()
	log.Println("shutting down...")

	grpcServer.GracefulStop()
	database.Checkpoint(db)

	log.Println("shutdown complete")
}

// pinnedUser picks the single-account name to run with: the --user flag wins,
// otherwise the LUNAR_USER environment variable (which is how the Docker
// Compose stack configures it, having no place to pass flags). The second
// return value names the source so errors point at what the operator set.
func pinnedUser(flagValue string) (name, origin string) {
	if flagValue != "" {
		return flagValue, "--user"
	}
	return os.Getenv(pinnedUserEnv), pinnedUserEnv
}

// resolvePinnedUser turns the pinned player name into a user id, failing fast
// with the list of available accounts when it does not match exactly.
func resolvePinnedUser(s *sqlite.SQLiteStore, name, origin string) int64 {
	// "--user "Alice"" for the flag, "LUNAR_USER="Alice"" for the env var.
	sep := " "
	if origin == pinnedUserEnv {
		sep = "="
	}
	where := origin + sep + strconv.Quote(name)

	userId, err := s.GetUserByName(name)
	if err == nil {
		log.Printf("[!!] single-account mode (%s): every client login resolves to %q (user_id=%d)", origin, name, userId)
		return userId
	}
	if !errors.Is(err, store.ErrNotFound) {
		log.Fatalf("%s: %v", where, err)
	}

	accounts, listErr := s.ListAccounts()
	if listErr != nil {
		log.Fatalf("%s: no account with that name (listing accounts failed: %v)", where, listErr)
	}
	var b strings.Builder
	b.WriteString(where)
	b.WriteString(": no account with that in-game name.")
	if len(accounts) == 0 {
		b.WriteString(" The database has no accounts yet — start the server without it, create the account in-game, then restart with it.")
	} else {
		b.WriteString("\nKnown accounts:")
		for _, a := range accounts {
			label := a.Name
			if label == "" {
				label = "(no name set yet)"
			}
			b.WriteString("\n  user_id=")
			b.WriteString(strconv.FormatInt(a.UserId, 10))
			b.WriteString("  ")
			b.WriteString(label)
		}
	}
	log.Fatal(b.String())
	return 0
}
