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

- 🩺 **AutoDoctor (Diagnóstico Inteligente del Servidor):**
  - **Health Score (0 - 100):** Indicador de salud global (`🟢 Excelente`, `🟡 Precaución`, `🔴 Crítico`) en la cabecera del panel.
  - **Diagnóstico de Causa Raíz:** En lugar de solo mostrar errores, AutoDoctor cruza exit codes, inspección de contenedores, cgroups de memoria (Exit 137 OOM Killer), scripts faltantes (Exit 126/127), bucles continuos de reinicio y healthchecks fallidos.
  - **Auditoría de Almacenamiento:** Detecta espacio ocupado en imágenes dangling, volúmenes huérfanos y caché de construcción, permitiendo liberarlo con 1 tecla (<kbd>c</kbd>).
  - **Auditoría de Seguridad Rápida:** Detecta contenedores en modo privilegiado (`--privileged`), sockets montados (`/var/run/docker.sock`) y puertos de bases de datos sensibles (Postgres, MySQL, Redis, MongoDB) expuestos públicamente en `0.0.0.0`.
  - **Top 3 Acciones Prioritarias:** Responde a la pregunta clave: *"¿Cuáles son los problemas que debo atender primero y exactamente cómo solucionarlos?"*.
  - **Modo CLI Rápido (`--doctor`):** Ejecutable directamente en scripts o terminal remota sin abrir la interfaz interactiva.
- 🖥️ **Panel Interactivo por Defecto:** Al ejecutar `dockeretior` en tu shell (local o por **SSH**), se despliega de inmediato el dashboard con todos los contenedores en ejecución.
- 📐 **Diseño en Pantalla Dividida (*Split View*):**
  - **Margen Izquierdo:** Tabla interactiva de contenedores con indicadores de estado en color, nombres, imágenes, puertos e IDs.
  - **Margen Derecho (Ventanas ASCII):** Paneles delimitados por caracteres ASCII / rayitas que muestran en tiempo real las métricas del contenedor seleccionado:
    1. **Carga de CPU:** Porcentaje de uso, núcleos activos, barra de carga `[████░░░░]` y gráfico histórico de tendencia (*sparkline*).
    2. **Carga de Memoria:** Memoria usada vs límite del host, porcentaje, barra gráfica, caché inactiva y pico máximo.
    3. **Carga de Red (I/O):** Tráfico RX (entrante) y TX (saliente) con conteo de paquetes y flujo visual.
    4. **Resto de Usos y Estado:** Disco Block I/O (lectura/escritura), PIDs de procesos en kernel, IP y red Docker, estado y tiempo activo (*uptime*).
- ⌨️ **Banda de Menú Inferior (Teclas de Función F1 - F10):**
  - **F1 Ayuda:** Ventana emergente con todos los atajos y funciones.
  - **F2 Listar / Filtrar:** Alterna entre mostrar solo contenedores activos (*running*) o todos (activos, detenidos y pausados).
  - **F3 Logs:** Visor de registros en vivo con refresco en caliente.
  - **F4 Exec:** Abre una shell interactiva dentro del contenedor (`/bin/bash` o `/bin/sh`) y vuelve limpiamente al salir (`exit`).
  - **F5 Restart:** Reinicia el contenedor seleccionado.
  - **F6 Stop / Start:** Detiene o inicia el contenedor según su estado actual.
  - **F7 Pausar / Reanudar:** Pausa o reanuda los procesos del contenedor.
  - **F8 Borrar:** Cuadro de diálogo de confirmación para eliminar el contenedor, con soporte de borrado forzado (`f`).
  - **F9 Inspect:** Inspección detallada en JSON formateado.
  - **F10 Salir:** Cierra Dockeretior limpiamente y restaura tu terminal.
- ⚡ **Modo Supervisor PTY (*Hot-Toggle* opcional):** Disponible con la bandera `--supervisor` o `-s` para ejecutarse en segundo plano en sesiones SSH permanentes.

---

## ⌨️ Tabla de Atajos de Teclado

| Tecla de Función | Atajo Alternativo | Acción |
| :--- | :--- | :--- |
| <kbd>F1</kbd> | <kbd>?</kbd> o <kbd>1</kbd> | **Ayuda:** Ver ventana emergente con la guía de teclas |
| <kbd>F2</kbd> | <kbd>f</kbd> o <kbd>2</kbd> | **Listar / Filtrar:** Alternar entre *Solo activos* y *Todos los contenedores* |
| <kbd>F3</kbd> | <kbd>l</kbd> o <kbd>3</kbd> | **Logs:** Abrir visor de registros en vivo |
| <kbd>F4</kbd> | <kbd>e</kbd> o <kbd>4</kbd> | **Exec:** Entrar al contenedor con terminal interactiva (`bash`/`sh`) |
| <kbd>F5</kbd> | <kbd>r</kbd> o <kbd>5</kbd> | **Restart:** Reiniciar contenedor seleccionado |
| <kbd>F6</kbd> | <kbd>s</kbd> o <kbd>6</kbd> | **Stop / Start:** Detener contenedor activo o iniciar contenedor detenido |
| <kbd>F7</kbd> | <kbd>p</kbd> o <kbd>7</kbd> | **Pausar / Reanudar:** Pausar o reanudar procesos del contenedor |
| <kbd>F8</kbd> | <kbd>x</kbd> o <kbd>8</kbd> | **Borrar:** Eliminar contenedor (con confirmación `y` o forzar con `f`) |
| <kbd>F9</kbd> | <kbd>i</kbd> o <kbd>9</kbd> | **Inspect:** Inspeccionar configuración completa en JSON |
| <kbd>F10</kbd> | <kbd>q</kbd> o <kbd>0</kbd> o <kbd>Esc</kbd> | **Salir:** Cerrar Dockeretior y regresar al bash / SSH |
| <kbd>a</kbd> / <kbd>A</kbd> | | **AutoDoctor:** Diagnóstico de salud, causas raíz y recomendaciones |
| <kbd>c</kbd> | | **Compose / Limpiar:** Compose en panel principal o Limpieza de espacio en AutoDoctor |
| <kbd>↑</kbd> / <kbd>↓</kbd> | <kbd>k</kbd> / <kbd>j</kbd> | **Navegación:** Desplazar cursor sobre los contenedores y actualizar ventanas |

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

### 1. Panel de Control Directo (Por defecto)
Simplemente escribe en tu terminal o sesión SSH:

```bash
dockeretior
```

Se abrirá inmediatamente la pantalla dividida (*split view*) mostrando todos los contenedores en ejecución en el margen izquierdo y las 4 ventanas ASCII con las cargas de CPU, memoria, red y resto de usos en el margen derecho, con la botonera de funciones F1 - F10 en la parte inferior. Puedes pulsar <kbd>a</kbd> en cualquier momento para abrir **AutoDoctor** y revisar la salud del servidor.

### 2. AutoDoctor en Línea de Comandos (`--doctor`)
Para obtener un diagnóstico instantáneo de fallos, causas raíz y recomendaciones sin entrar a la TUI interactiva (ideal para scripts, alertas o inspección rápida por SSH):

```bash
dockeretior --doctor
```

### 3. Modo Supervisor Latente en Segundo Plano (*Hot-Toggle*)
Si deseas que Dockeretior permanezca invisible en segundo plano dentro de tu shell y se active únicamente al pulsar la combinación de teclas (`Ctrl+\` o `Ctrl+Alt+Espacio`):

```bash
dockeretior --supervisor
```

---

## 🌐 Integración Permanente por SSH

Para que **Dockeretior** esté disponible como supervisor transparente en cada una de tus sesiones remotas SSH:

Añade las siguientes líneas al final de tu archivo `~/.zprofile`, `~/.bash_profile` o `~/.bashrc`:

```bash
if [ -z "$DOCKERETIOR_ACTIVE" ] && [ -t 1 ]; then
    export DOCKERETIOR_ACTIVE=1
    exec /usr/local/bin/dockeretior --supervisor
fi
```

Al conectar por SSH a tu servidor, tu shell funcionará con total normalidad y podrás abrir Dockeretior al instante con `Ctrl + \` o `Ctrl + Alt + Espacio`. También podrás ejecutar `dockeretior` en cualquier momento para abrir el panel directamente.

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
