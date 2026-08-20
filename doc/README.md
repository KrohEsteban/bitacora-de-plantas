# 🌿 Bitácora de Plantas

Una aplicación web moderna para llevar un registro detallado de tus plantas, documenta su crecimiento con fotografías y mantén notas sobre sus cuidados.

## ¿Qué es?

Bitácora de Plantas es una aplicación web minimalista construida con Go y HTMX que te permite:

- **📝 Crear fichas** para cada una de tus plantas con nombre y descripción
- **📷 Subir múltiples imágenes** para documentar el crecimiento
- **🔍 Ver galería** de fotos de cada planta con navegación fluida
- **✏️ Editar información** de plantas existentes
- **🗑️ Gestionar imágenes** con eliminación individual
- **💾 Almacenamiento local** sin bases de datos, todo en el sistema de archivos

## Características Técnicas

- **Backend**: Go puro con librería estándar (net/http)
- **Frontend**: HTMX 2.0.10 + Alpine.js 3 + CSS personalizado
- **Datos**: Sistema de archivos (sin base de datos)
- **Imágenes**: Generación automática de miniaturas
- **Validación**: Sanitización de archivos y rutas para seguridad
- **UI**: Diseño responsivo con tema verde natural

## Cómo Ejecutar

### Usando Docker (Recomendado)

La forma más sencilla es usar Docker Compose:

```bash
# Clonar el repositorio y navegar al directorio
cd bitacora-plantas

# Ejecutar con Docker Compose
docker compose up -d

# La aplicación estará disponible en http://localhost:8080
```

### Verificar Estado

```bash
# Verificar que el contenedor está corriendo
docker compose ps

# Ver logs si es necesario
docker compose logs -f bitacora
```

### Detener la Aplicación

```bash
docker compose down
```

## Dónde se Guardan los Datos

Todos los datos de tus plantas se almacenan en el **volumen Docker nombrado `bitacora-data`**, montado en `/app/data` dentro del contenedor. Esto garantiza que las plantas y las fotos sobreviven a redeploys, rebuilds y checkouts limpios del repositorio, porque el volumen vive en el almacenamiento de Docker (independiente del directorio del proyecto).

Estructura interna de `/app/data` (dentro del volumen):

```
data/
└── plants/
    ├── rosa-del-jardin/
    │   ├── meta.json            # Información de la planta
    │   └── images/
    │       ├── foto1.jpg        # Imágenes originales (sin modificar)
    │       ├── foto2.png
    │       ├── .thumb/          # Miniaturas generadas (JPEG real, .jpg)
    │       │   ├── foto1.jpg
    │       │   └── foto2.jpg
    │       └── .display/        # Versiones de visualización (JPEG real, .jpg)
    │           ├── foto1.jpg
    │           └── foto2.jpg
    └── cactus-pequeno/
        ├── meta.json
        └── images/
```

**Importante**: El directorio `./data/` del host **ya no se usa** — las imágenes del contenedor viven únicamente en el volumen `bitacora-data`. El `.gitignore` sigue ignorando `data/` (por compatibilidad, el directorio local puede eliminarse sin afectar la app).

### Copia de Seguridad (Backup)

Para respaldar todos los datos del volumen en un archivo `.tar.gz`:

```bash
docker run --rm -v bitacora-data:/app/data -v $PWD:/backup alpine tar czf /backup/bitacora-data.tar.gz -C /app/data .
```

Para restaurar un backup en el volumen:

```bash
docker volume create bitacora-data
docker run --rm -v bitacora-data:/app/data -v $PWD:/backup alpine tar xzf /backup/bitacora-data.tar.gz -C /app/data
```

## Funcionalidades

### Crear Nueva Planta
- Haz clic en "Nueva Planta"
- Completa nombre y descripción (cuidados, riego, etc.)
- Opcionalmente sube imágenes iniciales
- Máximo 10MB por imagen (JPG, PNG, WebP)

### Gestionar Plantas Existentes
- Haz clic en cualquier tarjeta de planta para ver detalles
- Usa "Editar" para cambiar nombre y descripción
- Usa "Agregar Imágenes" para subir más fotos
- Haz clic en 🗑️ sobre cualquier imagen para eliminarla

### Navegación
- La vista principal muestra todas tus plantas en una cuadrícula
- Cada tarjeta muestra la primera imagen (o icono) y resumen
- Vista detalle muestra galería completa e información
- Botón "Volver" para regresar a la vista general

## Seguridad

La aplicación incluye varias medidas de seguridad:

- **Sanitización de archivos**: Solo permite imágenes válidas (JPG, PNG, WebP)
- **Validación de rutas**: Previene path traversal y acceso no autorizado
- **Límites de subida**: 10MB máximo por imagen
- **Detección de tipo MIME**: Verifica contenido real de archivos
- **Nombres únicos**: Evita conflictos de archivos automáticamente

## Desarrollo

Si quieres ejecutar localmente para desarrollo:

```bash
# Tener Go 1.23+ instalado
go mod download
go run main.go

# Aplicación disponible en http://localhost:8080
```

## Estructura del Proyecto

```
bitacora-plantas/
├── main.go              # Aplicación principal Go
├── templates/           # Templates HTML
│   ├── index.html       # Página principal
│   ├── plant-grid.html  # Cuadrícula de plantas
│   └── plant-detail.html # Vista detalle
├── static/
│   └── style.css        # Estilos CSS con tema verde
├── Dockerfile           # Configuración Docker
├── docker-compose.yml   # Orquestación Docker
├── go.mod              # Dependencias Go
└── README.md           # Este archivo
```

## Tecnologías Utilizadas

- **Go 1.23**: Backend robusto y eficiente
- **HTMX 2.0.10**: Interactividad sin JavaScript complejo
- **Alpine.js 3**: Reactividad ligera para modales
- **CSS Grid/Flexbox**: Layout responsivo moderno
- **Docker**: Contenedorización para fácil despliegue

## Créditos

Desarrollado como una demostración de aplicaciones web modernas usando tecnologías web fundamentales con un enfoque en la simplicidad y el rendimiento.