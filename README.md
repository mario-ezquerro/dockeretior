# ⚓ Dockeretior

<div align="center">

[![Go Report Card](https://goreportcard.com/badge/github.com/mario-ezquerro/dockeretior)](https://goreportcard.com/report/github.com/mario-ezquerro/dockeretior)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Go Version](https://img.shields.io/github/go-mod/go-version/mario-ezquerro/dockeretior)](go.mod)
[![Platform](https://img.shields.io/badge/Platform-Linux%20%7C%20macOS-informational.svg)](https://github.com/mario-ezquerro/dockeretior)

**Supervisor interactivo y residente de pseudo-terminal (PTY) para Docker y Docker Compose con activación en caliente (*Hot-Toggle*).**

[Características](#-características) • [Instalación](#-instalación) • [Uso y Atajos](#-uso-y-atajos) • [Integración SSH](#-integración-permanente-por-ssh) • [Licencia](#-licencia)

</div>

---

## 💡 ¿Qué es Dockeretior?

**Dockeretior** actúa como un proxy transparente de pseudo-terminal (PTY) entre tu conexión de terminal (o sesión remota por **SSH**) y tu shell habitual (`bash` o `zsh`).

Permanece **completamente latente e invisible** en segundo plano mientras trabajas normalmente en la línea de comandos. Cuando necesitas inspeccionar contenedores, depurar un servicio caído o revisar tus proyectos Compose, basta con pulsar:

$$\boxed{\text{Ctrl}} + \boxed{\text{Alt / Option}} + \boxed{\text{Espacio}}$$

Al instante, la shell actual se **congela en segundo plano** sin interrumpir ningún proceso, se conmuta al búfer alternativo del terminal (`\x1b[?1049h`), y se despliega una interfaz gráfica en terminal (TUI) de alto rendimiento construida con [Bubble Tea](https://github.com/charmbracelet/bubbletea), [Lip Gloss](https://github.com/charmbracelet/lipgloss) y el SDK oficial de Docker.

Al terminar o volver a pulsar la combinación, la pantalla vuelve de forma **100% limpia e intacta** al comando o cursor donde estabas.

---

## ✨ Características

- ⚡ **Alternancia en caliente (*Hot-Toggle*):** Entra y sal de la interfaz gráfica al milisegundo mediante `Ctrl + Alt + Espacio`.
- 🛡️ **Sin interrupciones:** No cierra subprocesos ni interrumpe comandos a medio teclear en la terminal.
- 📦 **Gestión de Contenedores:**
  - Inspección de estado en tiempo real (ID, nombre, imagen, puertos, estado).
  - Acciones de ciclo de vida con un solo toque: Detener (`s`), Iniciar (`a`), Reiniciar (`r`), Pausar/Reanudar (`p`).
  - Visor detallado de configuración e inspección en JSON (`d`).
- 📜 **Streaming de Logs en Vivo:** Visualización inmediata de las últimas líneas de logs de cualquier contenedor con opción de refresco en caliente (`l`).
- 🐙 **Escaneo Inteligente de Docker Compose:**
  - Detección automática de archivos `docker-compose.yml`, `docker-compose.yaml` y `compose.yml` en el directorio de trabajo.
  - Desglose visual de servicios, imágenes y puertos mapeados.
  - Ejecución de comandos Compose (`u` para `compose up -d`, `r` para restart, etc.).
- 📊 **Panel de Información del Host y Daemon:** Visión global del estado del motor Docker (versión, contenedores en ejecución/pausados, storage driver, CPU y memoria del sistema).
- 🧹 **Búfer ANSI Limpio:** Utiliza conmutación `smcup`/`rmcup` para garantizar cero residuos en el historial de comandos de tu terminal.

---

## ⌨️ Tabla de Atajos de Teclado

| Atajo | Contexto | Acción |
| :--- | :--- | :--- |
| <kbd>Ctrl</kbd> + <kbd>Alt</kbd> + <kbd>Espacio</kbd> | **Global** | **Alternar entre la Shell y Dockeretior (*Toggle*)** |
| <kbd>↑</kbd> / <kbd>↓</kbd> o <kbd>j</kbd> / <kbd>k</kbd> | Listas y Menús | Mover cursor arriba / abajo |
| <kbd>Enter</kbd> | Menú / Listas | Acceder al módulo o inspeccionar archivo |
| <kbd>s</kbd> | Contenedores | **Detener** contenedor seleccionado |
| <kbd>a</kbd> | Contenedores | **Iniciar** contenedor detenido |
| <kbd>r</kbd> | Contenedores / Logs / Compose | **Reiniciar** contenedor o proyecto / Refrescar logs |
| <kbd>p</kbd> | Contenedores | **Pausar / Reanudar** procesos del contenedor |
| <kbd>l</kbd> | Contenedores | Abrir visor de **logs en tiempo real** |
| <kbd>d</kbd> | Contenedores | **Inspeccionar** contenedor (formato JSON formateado) |
| <kbd>u</kbd> | Compose | Ejecutar `docker compose up -d` en el proyecto |
| <kbd>q</kbd> o <kbd>Esc</kbd> | Vistas / Submenús | Volver al nivel anterior / Ocultar Dockeretior |

---

## 🛠️ Requisitos

- **Sistema Operativo:** Linux o macOS.
- **Go:** Versión `1.22` o superior (para compilar desde código fuente).
- **Docker Engine:** Socket de Docker accesible (`/var/run/docker.sock`).
- Permisos adecuados en el socket (`sudo usermod -aG docker $USER`).

---

## 🚀 Instalación

### Opción 1: Compilar desde código fuente (Recomendada)

```bash
# 1. Clonar el repositorio
git clone https://github.com/mario-ezquerro/dockeretior.git
cd dockeretior

# 2. Compilar binario
make build

# 3. Instalar en el sistema (/usr/local/bin)
sudo make install
```

### Opción 2: Vía `go install`

```bash
go install github.com/mario-ezquerro/dockeretior/cmd/dockeretior@latest
```

---

## 💻 Modo de Uso

### 1. Inicio como Supervisor Residente (Por defecto)
Simplemente escribe en tu terminal:

```bash
dockeretior
```

A partir de ese momento, tu sesión habitual de `bash` o `zsh` continuará funcionando normalmente. Pulsa `Ctrl + Alt + Espacio` cuando desees desplegar el panel.

### 2. Modo TUI Directo (Sin proxy PTY)
Si deseas abrir únicamente el panel gráfico de una sola vez:

```bash
dockeretior --tui
```

---

## 🌐 Integración Permanente por SSH

Para que **Dockeretior** proteja y supervise automáticamente cada una de tus conexiones SSH:

Añade las siguientes líneas al final de tu archivo `~/.zprofile`, `~/.bash_profile` o `~/.bashrc`:

```bash
if [ -z "$DOCKERETIOR_ACTIVE" ] && [ -t 1 ]; then
    export DOCKERETIOR_ACTIVE=1
    exec /usr/local/bin/dockeretior
fi
```

Al conectar por SSH a tu servidor, el supervisor arrancará en milisegundos de forma imperceptible y tendrás el panel disponible con `Ctrl + Alt + Espacio` en todo momento.

---

## 🤝 Contribuciones

Las contribuciones, issues y pull requests son bienvenidas. Si tienes ideas para nuevos atajos, métricas o integraciones, ¡abre un issue en GitHub!

1. Haz un Fork del proyecto.
2. Crea tu rama de características (`git checkout -b feat/nueva-funcionalidad`).
3. Confirma tus cambios (`git commit -m 'feat: añadir soporte para métricas avanzadas'`).
4. Haz push a la rama (`git push origin feat/nueva-funcionalidad`).
5. Abre un Pull Request.

---

## 📄 Licencia

Este proyecto está bajo la Licencia **MIT**. Consulta el archivo [LICENSE](LICENSE) para más detalles.

Copyright (c) 2026 **Mario Ezquerro Saenz**
