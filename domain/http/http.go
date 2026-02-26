package http

import (
	"context"
	"crypto/tls"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	"github.com/vanyayudin26/medcolosma_parser/v2"
	"github.com/vanyayudin26/medcolosma_schedule_api/config"
	"github.com/vanyayudin26/medcolosma_schedule_api/domain/http/handler"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/acme/autocert"
)

func apiMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "*")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		if r.Method == "POST" {
			r.Method = "GET"
		}

		next.ServeHTTP(w, r)
	})
}

func Start(cfg config.HTTP, sch *hmtpk_parser.Controller) error {
	appHandler := apiMiddleware(handler.Router(cfg, sch))

	// --- ФОНОВЫЙ ЧЕКЕР ОБНОВЛЕНИЯ РАСПИСАНИЯ ---
	// Запускаем его ДО того, как ListenAndServe заблокирует выполнение
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		var lastKnownDate string

		for range ticker.C {
			ctx := context.Background()
			currentDate, err := sch.GetLastUpdateDate(ctx)

			if err == nil && currentDate != "" {
				if lastKnownDate == "" {
					lastKnownDate = currentDate
					log.Infof("Фоновый чекер запущен. Текущая дата расписания: %s", lastKnownDate)
				} else if currentDate != lastKnownDate {
					log.Infof("ОБНАРУЖЕНО НОВОЕ РАСПИСАНИЕ! Было: %s, Стало: %s. Очищаем кэш...", lastKnownDate, currentDate)

					if err := sch.ClearCache(ctx); err != nil {
						log.Errorf("Ошибка при очистке кэша: %v", err)
					} else {
						log.Info("Кэш успешно очищен.")
					}

					lastKnownDate = currentDate
				}
			} else if err != nil {
				log.Warnf("Ошибка проверки даты расписания: %v", err)
			}
		}
	}()
	// --- КОНЕЦ БЛОКА ЧЕКЕРА ---

	if cfg.HTTPSAddress == "" {
		log.Tracef("http server: %s%s", cfg.Domain, cfg.HTTPAddress)
		return http.ListenAndServe(cfg.HTTPAddress, appHandler)
	}

	certManager := autocert.Manager{
		Prompt: autocert.AcceptTOS,
		Cache:  autocert.DirCache("/tmp/cache-golang-autocert"),
	}

	if cfg.Domain != "localhost" {
		certManager.HostPolicy = autocert.HostWhitelist(cfg.Domain)
	}

	if u, _ := user.Current(); u != nil {
		dir := filepath.Join(os.TempDir(), "cache-golang-autocert-"+u.Username)
		if os.MkdirAll(dir, 0700) == nil {
			certManager.Cache = autocert.DirCache(dir)
		}
	}

	server := &http.Server{
		Addr:    cfg.HTTPSAddress,
		Handler: appHandler,
		TLSConfig: &tls.Config{
			GetCertificate:   certManager.GetCertificate,
			CurvePreferences: []tls.CurveID{tls.X25519, tls.CurveP256},
		},
		IdleTimeout:  time.Minute,
		ReadTimeout:  1 * time.Minute,
		WriteTimeout: 1 * time.Minute,
	}

	go func() {
		log.Fatal(http.ListenAndServe(cfg.HTTPAddress, certManager.HTTPHandler(appHandler)))
	}()

	return server.ListenAndServeTLS("", "")
}