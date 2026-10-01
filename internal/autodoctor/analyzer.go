package autodoctor

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/mario-ezquerro/dockeretior/internal/docker"
)

// RunAudit performs a full system, container, storage, and security health check.
func RunAudit(ctx context.Context, cli *docker.Client) (*HealthReport, error) {
	if cli == nil {
		return nil, fmt.Errorf("docker client not available")
	}

	report := &HealthReport{
		GeneratedAt: time.Now(),
	}

	var issues []Issue

	// 1. Inspect Docker Host and Server Info
	serverInfo, err := cli.ServerInfo(ctx)
	engineStatus := SeverityOK
	engineSummary := "Docker Daemon activo y respondiendo normalmente."
	if err != nil {
		engineStatus = SeverityCritical
		engineSummary = fmt.Sprintf("Error conectando con el socket Docker: %v", err)
		issues = append(issues, Issue{
			ID:             "issue_engine_down",
			Category:       CategoryHost,
			Severity:       SeverityCritical,
			Title:          "Docker Daemon no responde",
			Target:         "Docker Host",
			Description:    "No se puede comunicar con el motor Docker local.",
			RootCause:      err.Error(),
			Recommendation: "Verifica que el servicio Docker está iniciado (systemctl status docker).",
			Timestamp:      time.Now(),
		})
	} else {
		engineSummary = fmt.Sprintf("Docker v%s (%s), %d CPUs, %.1f GB RAM.",
			serverInfo.ServerVersion, serverInfo.OperatingSystem,
			serverInfo.NCPU, float64(serverInfo.MemTotal)/(1024*1024*1024))
	}

	// 2. Containers Inspection and Deep Diagnostics
	containers, _ := cli.ListContainers(ctx, true)
	report.ContainersTotal = len(containers)

	var runningCount, stoppedCount int
	var restartLoopCount, oomCount, securityCount int

	for _, c := range containers {
		isRun := strings.ToLower(c.State) == "running"
		if isRun {
			runningCount++
		} else {
			stoppedCount++
		}

		name := "unnamed"
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		// Inspect container in detail
		inspect, err := cli.InspectContainer(ctx, c.ID)
		if err != nil {
			continue
		}

		// Check 1: Exit Code 137 / OOM Killer
		if inspect.State != nil && (inspect.State.ExitCode == 137 || inspect.State.OOMKilled) {
			oomCount++
			memLimitStr := "Sin límite específico"
			if inspect.HostConfig != nil && inspect.HostConfig.Memory > 0 {
				memLimitStr = docker.FormatBytes(uint64(inspect.HostConfig.Memory))
			}
			issues = append(issues, Issue{
				ID:             fmt.Sprintf("oom_%s", c.ID[:min(12, len(c.ID))]),
				Category:       CategoryContainer,
				Severity:       SeverityCritical,
				Title:          fmt.Sprintf("Contenedor '%s' terminado por OOM Killer (Exit 137)", name),
				Target:         name,
				Description:    fmt.Sprintf("El proceso principal en '%s' fue eliminado por falta de memoria.", name),
				RootCause:      fmt.Sprintf("Presión excesiva de memoria física. El contenedor alcanzó el límite cgroup fijado (%s).", memLimitStr),
				Recommendation: "Incrementa el parámetro 'memory' en compose.yml o investiga fugas de memoria en la aplicación.",
				Timestamp:      time.Now(),
			})
		}

		// Check 2: Restart Loop (reiniciándose continuamente)
		if inspect.RestartCount > 3 || (inspect.State != nil && inspect.State.Restarting) {
			restartLoopCount++
			lastErr := ""
			if inspect.State != nil {
				lastErr = inspect.State.Error
				if lastErr == "" && inspect.State.ExitCode != 0 {
					lastErr = fmt.Sprintf("Exit Code %d", inspect.State.ExitCode)
				}
			}
			issues = append(issues, Issue{
				ID:             fmt.Sprintf("restart_loop_%s", c.ID[:min(12, len(c.ID))]),
				Category:       CategoryContainer,
				Severity:       SeverityCritical,
				Title:          fmt.Sprintf("Bucle continuo de reinicios en '%s' (%d caídas)", name, inspect.RestartCount),
				Target:         name,
				Description:    fmt.Sprintf("El servicio '%s' se está reiniciando constantemente.", name),
				RootCause:      fmt.Sprintf("El contenedor muere a los pocos segundos de arrancar. Causa reportada: %s.", lastErr),
				Recommendation: "Revisa los registros recientes del contenedor (pulsa F3) para depurar la excepción o configuración errónea.",
				Timestamp:      time.Now(),
			})
		}

		// Check 3: Entrypoint / Binary Not Found (Exit 126 / 127)
		if inspect.State != nil && (inspect.State.ExitCode == 126 || inspect.State.ExitCode == 127) {
			issues = append(issues, Issue{
				ID:             fmt.Sprintf("exec_err_%s", c.ID[:min(12, len(c.ID))]),
				Category:       CategoryContainer,
				Severity:       SeverityCritical,
				Title:          fmt.Sprintf("Ejecutable o script no encontrado en '%s' (Exit %d)", name, inspect.State.ExitCode),
				Target:         name,
				Description:    "El contenedor no pudo iniciar porque el comando CMD o ENTRYPOINT falló.",
				RootCause:      "El archivo binario o script especificado no existe dentro de la imagen o no tiene permisos de ejecución ('chmod +x').",
				Recommendation: "Verifica la ruta en el Dockerfile o docker-compose.yml y asegúrate de que el intérprete (/bin/sh o /bin/bash) existe en la imagen base.",
				Timestamp:      time.Now(),
			})
		}

		// Check 4: Unhealthy Healthcheck
		if inspect.State != nil && inspect.State.Health != nil && inspect.State.Health.Status == "unhealthy" {
			issues = append(issues, Issue{
				ID:             fmt.Sprintf("unhealthy_%s", c.ID[:min(12, len(c.ID))]),
				Category:       CategoryContainer,
				Severity:       SeverityWarning,
				Title:          fmt.Sprintf("Healthcheck en fallo continuo en '%s'", name),
				Target:         name,
				Description:    "El contenedor está en ejecución pero su comprobación de salud está fallando.",
				RootCause:      fmt.Sprintf("El comando de comprobación configurado devolvió fallo (%d fallos consecutivos).", inspect.State.Health.FailingStreak),
				Recommendation: "Comprueba el endpoint interno o script de healthcheck definido en el contenedor.",
				Timestamp:      time.Now(),
			})
		}

		// Check 5: Security - Privileged Container
		if inspect.HostConfig != nil && inspect.HostConfig.Privileged {
			securityCount++
			issues = append(issues, Issue{
				ID:             fmt.Sprintf("sec_priv_%s", c.ID[:min(12, len(c.ID))]),
				Category:       CategorySecurity,
				Severity:       SeverityCritical,
				Title:          fmt.Sprintf("Contenedor '%s' en modo privilegiado (--privileged)", name),
				Target:         name,
				Description:    "El contenedor tiene acceso irrestricto a los dispositivos y kernel del host.",
				RootCause:      "Modo privilegiado habilitado, eliminando el aislamiento de seguridad estándar de Docker.",
				Recommendation: "Retira 'privileged: true' y asigna solo las capacidades Linux requeridas ('cap_add').",
				Timestamp:      time.Now(),
			})
		}

		// Check 6: Security - Docker Socket Mounted
		for _, m := range inspect.Mounts {
			if strings.Contains(m.Source, "docker.sock") || strings.Contains(m.Destination, "docker.sock") {
				securityCount++
				issues = append(issues, Issue{
					ID:             fmt.Sprintf("sec_sock_%s", c.ID[:min(12, len(c.ID))]),
					Category:       CategorySecurity,
					Severity:       SeverityWarning,
					Title:          fmt.Sprintf("Socket Docker montado en '%s'", name),
					Target:         name,
					Description:    "El contenedor tiene acceso al socket de control /var/run/docker.sock del host.",
					RootCause:      "Montaje de volumen del socket del motor Docker, permitiendo control sobre otros contenedores.",
					Recommendation: "Si el contenedor no requiere administrar Docker, elimina el montaje de docker.sock.",
					Timestamp:      time.Now(),
				})
				break
			}
		}

		// Check 7: Security - Sensitive Database Ports Exposed on 0.0.0.0
		sensitivePorts := map[int]string{
			5432:  "PostgreSQL",
			3306:  "MySQL/MariaDB",
			6379:  "Redis",
			27017: "MongoDB",
		}
		for _, p := range c.Ports {
			if dbName, ok := sensitivePorts[int(p.PrivatePort)]; ok {
				if p.PublicPort > 0 && (p.IP == "0.0.0.0" || p.IP == "::") {
					securityCount++
					issues = append(issues, Issue{
						ID:             fmt.Sprintf("sec_port_%s_%d", c.ID[:min(12, len(c.ID))], p.PublicPort),
						Category:       CategorySecurity,
						Severity:       SeverityWarning,
						Title:          fmt.Sprintf("Puerto de %s expuesto en 0.0.0.0 (%d:%d) en '%s'", dbName, p.PublicPort, p.PrivatePort, name),
						Target:         name,
						Description:    fmt.Sprintf("El puerto de %s es accesible públicamente desde cualquier IP.", dbName),
						RootCause:      "Mapeo de puertos sin restricción de interfaz local (0.0.0.0 permite conexiones directas externas).",
						Recommendation: fmt.Sprintf("Mapea a '127.0.0.1:%d:%d' o utiliza la red interna Docker para comunicar servicios.", p.PublicPort, p.PrivatePort),
						Timestamp:      time.Now(),
					})
				}
			}
		}

		// Check 8: Long-Stopped Container (Contenedores abandonados)
		if strings.ToLower(c.State) == "exited" && inspect.State != nil {
			finishedAt := inspect.State.FinishedAt
			if parsedTime, err := time.Parse(time.RFC3339Nano, finishedAt); err == nil {
				if time.Since(parsedTime) > 14*24*time.Hour {
					issues = append(issues, Issue{
						ID:             fmt.Sprintf("old_stopped_%s", c.ID[:min(12, len(c.ID))]),
						Category:       CategoryContainer,
						Severity:       SeverityInfo,
						Title:          fmt.Sprintf("Contenedor '%s' inactivo desde hace más de 14 días", name),
						Target:         name,
						Description:    fmt.Sprintf("Detenido el %s.", parsedTime.Format("02/01/2006")),
						RootCause:      "Contenedor fuera de servicio conservado en disco.",
						Recommendation: "Si ya no se utiliza, elimínalo con F8 para liberar espacio y recursos.",
						Timestamp:      time.Now(),
					})
				}
			}
		}
	}

	report.ContainersRunning = runningCount
	report.ContainersStopped = stoppedCount

	// 3. Storage and Disk Footprint Analysis
	du, err := cli.DiskUsage(ctx)
	if err == nil {
		var imgTotal, imgReclaim int64
		var danglingImgs int
		for _, img := range du.Images {
			imgTotal += img.Size
			if img.Containers == 0 {
				imgReclaim += img.Size
				danglingImgs++
			}
		}

		var volTotal, volReclaim int64
		var orphanedVols int
		for _, vol := range du.Volumes {
			if vol.UsageData != nil {
				volTotal += vol.UsageData.Size
				if vol.UsageData.RefCount == 0 {
					volReclaim += vol.UsageData.Size
					orphanedVols++
				}
			}
		}

		var bcTotal, bcReclaim int64
		for _, bc := range du.BuildCache {
			bcTotal += bc.Size
			if !bc.InUse {
				bcReclaim += bc.Size
			}
		}

		totalReclaim := imgReclaim + volReclaim + bcReclaim
		totalDocker := imgTotal + volTotal + bcTotal

		report.Storage = StorageAudit{
			ImagesTotalBytes:      imgTotal,
			ImagesReclaimable:     imgReclaim,
			ImagesCount:           len(du.Images),
			DanglingImagesCount:   danglingImgs,
			VolumesTotalBytes:     volTotal,
			VolumesReclaimable:    volReclaim,
			VolumesCount:          len(du.Volumes),
			OrphanedVolumesCount:  orphanedVols,
			BuildCacheTotal:       bcTotal,
			BuildCacheReclaimable: bcReclaim,
			TotalReclaimableBytes: totalReclaim,
			TotalDockerBytes:      totalDocker,
		}

		// Storage issue if more than 5 GB can be reclaimed
		if totalReclaim > 5*1024*1024*1024 {
			sev := SeverityWarning
			if totalReclaim > 20*1024*1024*1024 {
				sev = SeverityCritical
			}
			issues = append(issues, Issue{
				ID:                 "storage_reclaimable_high",
				Category:           CategoryStorage,
				Severity:           sev,
				Title:              fmt.Sprintf("Espacio recuperable significativo en Docker (%s)", docker.FormatBytes(uint64(totalReclaim))),
				Target:             "Docker Storage",
				Description:        fmt.Sprintf("Docker está reteniendo %s en recursos sin uso.", docker.FormatBytes(uint64(totalReclaim))),
				RootCause:          fmt.Sprintf("Acumulación de %d imágenes sin contenedor, %d volúmenes huérfanos y caché de construcción.", danglingImgs, orphanedVols),
				Recommendation:     "Ejecuta una limpieza asistida con la tecla 'c' en AutoDoctor para recuperar espacio inmediatamente.",
				RemediationCommand: "docker system prune -a --volumes",
				Timestamp:          time.Now(),
			})
		}
	}

	// 4. Subsystems Status Generation
	storageSev := SeverityOK
	storageSummary := fmt.Sprintf("Espacio Docker optimizado (%s en uso).", docker.FormatBytes(uint64(report.Storage.TotalDockerBytes)))
	if report.Storage.TotalReclaimableBytes > 20*1024*1024*1024 {
		storageSev = SeverityCritical
		storageSummary = fmt.Sprintf("%s recuperables en caché e imágenes sin uso.", docker.FormatBytes(uint64(report.Storage.TotalReclaimableBytes)))
	} else if report.Storage.TotalReclaimableBytes > 5*1024*1024*1024 {
		storageSev = SeverityWarning
		storageSummary = fmt.Sprintf("%s recuperables en imágenes/volúmenes huérfanos.", docker.FormatBytes(uint64(report.Storage.TotalReclaimableBytes)))
	}

	containersSev := SeverityOK
	containersSummary := fmt.Sprintf("%d contenedores activos y estables.", runningCount)
	if restartLoopCount > 0 || oomCount > 0 {
		containersSev = SeverityCritical
		containersSummary = fmt.Sprintf("%d en bucle de reinicios o terminados por OOM.", restartLoopCount+oomCount)
	} else if stoppedCount > 0 {
		containersSummary = fmt.Sprintf("%d activos, %d detenidos.", runningCount, stoppedCount)
	}

	secSev := SeverityOK
	secSummary := "Aislamiento y configuración de puertos estándar."
	if securityCount > 0 {
		secSev = SeverityWarning
		secSummary = fmt.Sprintf("%d advertencias de seguridad identificadas.", securityCount)
	}

	report.Subsystems = []SubsystemStatus{
		{Name: "Docker Engine", Status: engineStatus, Summary: engineSummary},
		{Name: "Contenedores", Status: containersSev, Summary: containersSummary},
		{Name: "Almacenamiento", Status: storageSev, Summary: storageSummary},
		{Name: "Seguridad", Status: secSev, Summary: secSummary},
	}

	// 5. Calculate Health Score (0 - 100)
	score := 100
	for _, iss := range issues {
		switch iss.Severity {
		case SeverityCritical:
			score -= 15
		case SeverityWarning:
			score -= 5
		case SeverityInfo:
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	report.HealthScore = score

	if score >= 85 {
		report.HealthLabel = "Excelente"
		report.HealthBadge = "🟢"
	} else if score >= 65 {
		report.HealthLabel = "Precaución"
		report.HealthBadge = "🟡"
	} else {
		report.HealthLabel = "Crítico"
		report.HealthBadge = "🔴"
	}

	// 6. Sort and Select Top 3 Prioritized Actions
	sort.SliceStable(issues, func(i, j int) bool {
		// Critical before Warning, Warning before Info
		priority := map[Severity]int{
			SeverityCritical: 3,
			SeverityWarning:  2,
			SeverityInfo:     1,
		}
		if priority[issues[i].Severity] != priority[issues[j].Severity] {
			return priority[issues[i].Severity] > priority[issues[j].Severity]
		}
		return issues[i].Category < issues[j].Category
	})

	report.Issues = issues
	if len(issues) > 3 {
		report.TopActions = issues[:3]
	} else {
		report.TopActions = issues
	}

	return report, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
