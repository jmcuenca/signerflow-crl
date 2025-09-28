package scheduler

import (
	"log"
	"signerflow-crl-service/services"

	"github.com/robfig/cron/v3"
)

type Scheduler struct {
	cron       *cron.Cron
	crlService *services.CRLService
	crlURLsFile string
}

func NewScheduler(crlService *services.CRLService, crlURLsFile string) *Scheduler {
	c := cron.New(cron.WithSeconds())

	return &Scheduler{
		cron:        c,
		crlService:  crlService,
		crlURLsFile: crlURLsFile,
	}
}

func (s *Scheduler) Start() error {
	_, err := s.cron.AddFunc("0 */10 * * * *", s.processCRLs)
	if err != nil {
		return err
	}

	_, err = s.cron.AddFunc("0 0 */6 * * *", s.cleanupCaches)
	if err != nil {
		return err
	}

	s.cron.Start()
	log.Println("Scheduler iniciado: procesamiento de CRLs cada 10 minutos")

	go s.initialProcessing()

	return nil
}

func (s *Scheduler) Stop() {
	s.cron.Stop()
	log.Println("Scheduler detenido")
}

func (s *Scheduler) processCRLs() {
	log.Println("Iniciando procesamiento programado de CRLs...")

	err := s.crlService.ProcessAllCRLs(s.crlURLsFile)
	if err != nil {
		log.Printf("Error en procesamiento programado de CRLs: %v", err)
	} else {
		log.Println("Procesamiento programado de CRLs completado exitosamente")
	}
}

func (s *Scheduler) cleanupCaches() {
	log.Println("Ejecutando limpieza de cache programada...")
}

func (s *Scheduler) initialProcessing() {
	log.Println("Ejecutando procesamiento inicial de CRLs...")

	err := s.crlService.ProcessAllCRLs(s.crlURLsFile)
	if err != nil {
		log.Printf("Error en procesamiento inicial de CRLs: %v", err)
	} else {
		log.Println("Procesamiento inicial de CRLs completado exitosamente")
	}
}

func (s *Scheduler) TriggerManualUpdate() {
	log.Println("Ejecutando actualización manual de CRLs...")
	go s.processCRLs()
}