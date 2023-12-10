<div align="center">
      <img src="resources/logo.png" alt="Logotipo de la interfaz de usuario de Nginx">
</div>

# Interfaz de usuario (UI) de Nginx

Otra UI web de Nginx, desarrollada por [0xJacky](https://jackyu.cn/) y [Hintay](https://blog.kugeek.com/).

[![Build and Publish](https://github.com/0xJacky/nginx-ui/actions/workflows/build.yml/badge.svg)](https://github.com/0xJacky/nginx-ui/actions/workflows/build.yml)

## Documentación
Para consultar la documentación, visite [nginxui.com](https://nginxui.com).

## Stargazers en el tiempo

[![Stargazers over time](https://starchart.cc/0xJacky/nginx-ui.svg)](https://starchart.cc/0xJacky/nginx-ui)


[English](README.md) | Español | [简体中文](README-zh_CN.md) | [繁體中文](README-zh_TW.md)

<details>
  <summary>Tabla de Contenidos</summary>
  <ol>
    <li>
      <a href="#about-the-project">Sobre el proyecto</a>
      <ul>
        <li><a href="#demo">Demostración</a></li>
        <li><a href="#features">Características</a></li>
        <li><a href="#internationalization">Internacionalización</a></li>
        <li><a href="#built-with">Desarrollado con</a></li>
      </ul>
    </li>
    <li>
      <a href="#getting-started">Cómo empezar</a>
      <ul>
        <li><a href="#before-use">Antes de usar</a></li>
        <li><a href="#installation">Instalación</a></li>
        <li>
          <a href="#usage">Uso</a>
          <ul>
            <li><a href="#from-executable">Desde el ejecutable</a></li>
            <li><a href="#with-systemd">Con Systemd</a></li>
            <li><a href="#with-docker">Con Docker</a></li>
          </ul>
        </li>
      </ul>
    </li>
    <li>
      <a href="#manual-build">Compilación manual</a>
      <ul>
        <li><a href="#prerequisites">Prerequisitos</a></li>
        <li><a href="#build-app">Compilación del Frontend</a></li>
        <li><a href="#build-backend">Compilación del Backend</a></li>
      </ul>
    </li>
    <li>
      <a href="#script-for-linux">Script para Linux</a>
      <ul>
        <li><a href="#basic-usage">Uso Básico</a></li>
        <li><a href="#more-usage">Uso Avanzado</a></li>
      </ul>
    </li>
    <li><a href="#example-of-nginx-reverse-proxy-configuration">Ejemplo de configuración de proxy reverso de Nginx</a></li>
    <li><a href="#contributing">Contribuir</a></li>
    <li><a href="#license">Licencia</a></li>
  </ol>
</details>


## Sobre el proyecto

![Dashboard](resources/screenshots/dashboard_en.png)

### Demostración
URL：[https://demo.nginxui.com](https://demo.nginxui.com)
- Nombre de usuario: admin
- Contraseña: admin

### Características

- Estadísticas en línea para los indicadores del servidor, como el uso de la CPU, el uso de la memoria, el promedio de carga y el uso del disco.
- Asistente de ChatGPT en línea
- Despliegue con un solo clic y renovación automática de certificados Let's Encrypt.
- Edición de las configuraciones de sitios web con nuestro **NgxConfigEditor** de diseño propio, que es un editor de bloques fácil de usar para configuraciones de nginx o con el **Editor de código Ace** que admite resaltar la sintaxis de configuración de nginx.
- Visualización en línea los registros de Nginx
- Escrito en Go y Vue, distribuído en un único binario ejecutable.
- Prueba automática del archivo de configuración y recarga de nginx después de guardar la configuración.
- Terminal Web
- Modo oscuro
- Diseño web adaptable

### Internacionalización

- Inglés
- Chino Simplificado
- Chino Tradicional

Aceptamos traducciones a cualquier idioma.

### Desarrollado con

- [El lenguaje de programación Go](https://go.dev)
- [Gin Web Framework](https://gin-gonic.com)
- [GORM](http://gorm.io)
- [Vue 3](https://v3.vuejs.org)
- [Vite](https://vitejs.dev)
- [TypeScript](https://www.typescriptlang.org/)
- [Ant Design Vue](https://antdv.com)
- [vue3-gettext](https://github.com/jshmrtn/vue3-gettext)
- [vue3-ace-editor](https://github.com/CarterLi/vue3-ace-editor)
- [Gonginx](https://github.com/tufanbarisyildirim/gonginx)

## Cómo empezar

### Antes de usar

La UI de Nginx sigue la configuración estándar de archivos de un servidor web de Debian. Los archivos de configuración del sitio creados se colocarán en la carpeta `sites-available` que se encuentra dentro de la carpeta de configuración de Nginx (detectada automáticamente). Los archivos de configuración para un sitio habilitado crearán un soft link en la carpeta `sites-enabled`. Es posible que deba ajustar la forma en que se organizan los archivos de configuración.

Para sistemas que no sean Debian (y Ubuntu), es posible que deba cambiar el contenido del archivo de configuración `nginx.conf` al estilo Debian como se muestra a continuación.

```nginx
http {
	# ...
	include /etc/nginx/conf.d/*.conf;
	include /etc/nginx/sites-enabled/*;
}
```

Para más información: [debian/conf/nginx.conf](https://salsa.debian.org/nginx-team/nginx/-/blob/master/debian/conf/nginx.conf#L59-L60)

### Instalación

La UI de Nginx está disponible en las siguientes plataformas:

- Mac OS X 10.10 Yosemite y posterior (amd64 / arm64)
- Linux 2.6.23 and later (x86 / amd64 / arm64 / armv5 / armv6 / armv7)
  - Incluyendo pero no limitado a Debian 7 / 8, Ubuntu 12.04 / 14.04 and later, CentOS 6 / 7, Arch Linux
- FreeBSD
- OpenBSD
- Dragonfly BSD
- Openwrt

Puede visitar [latest release](https://github.com/0xJacky/nginx-ui/releases/latest) para descargar la última distribución, o simplemente usar los [scripts de instalación para Linux](#script-for-linux).

### Uso

Al ejecutar por primera vez la UI de Nginx, visite en su navegador `http://<your_server_ip>:<listen_port>/install` para completar las configuraciones posteriores.

#### Desde el ejecutable
**Ejecutar UI Nginx en la terminal**

```shell
nginx-ui -config app.ini
```
Presione `Control+C` en la terminal para salir de la UI de Nginx.

**Ejecutar UI Nginx en Segundo plano**

```shell
nohup ./nginx-ui -config app.ini &
```
Detenga la UI de Nginx con el siguiente comando.

```shell
kill -9 $(ps -aux | grep nginx-ui | grep -v grep | awk '{print $2}')
```
#### Con Systemd
Si está utilizando el [script de instalación para Linux](#script-for-linux), la UI de Nginx se instalará como el servicio `nginx-ui` en systemd. Utilice el comando `systemctl` para controlarlo.

**Iniciar UI Nginx**

```shell
systemctl start nginx-ui
```
**Detener UI Nginx**

```shell
systemctl stop nginx-ui
```
**Reiniciar UI Nginx**

```shell
systemctl restart nginx-ui
```

#### Con Docker
Nuestra imagen dpcker [uozi/nginx-ui:latest](https://hub.docker.com/r/uozi/nginx-ui) se basa en la última imagen nginx y se puede usar para reemplazar Nginx en el host. Puede realizar el cambio fácilmente publicando los puertos 80 y 443 del contenedor en el host.


##### Nota
1. Cuando utilice este contenedor por primera vez, asegúrese de que el volumen mapeado a /etc/nginx esté vacío.
2. Si desea incluir archivos estáticos, puede mapear directorios al contenedor.

**Ejemplo de desplegado Docker**

```bash
docker run -dit \
  --name=nginx-ui \
  --restart=always \
  -e TZ=Asia/Shanghai \
  -v /mnt/user/appdata/nginx:/etc/nginx \
  -v /mnt/user/appdata/nginx-ui:/etc/nginx-ui \
  -v /var/www:/var/www \
  -p 8080:80 -p 8443:443 \
  uozi/nginx-ui:latest
```

## Compilación manual

En plataformas que no tienen una versión de compilación oficial, pueden compilarse manualmente.

### Prerequisitos

- Make

- Golang 1.19+

- node.js 18+

  ```shell
  npx browserslist@latest --update-db
  ```

### Compilación del Frontend

Ejecute el siguiente comando en el directorio `app`.

```shell
pnpm install
pnpm build
```

### Compilación del Backend

Primero compile la interfaz y luego ejecute el siguiente comando en el directorio raíz del proyecto.

```shell
go build -o nginx-ui -v main.go
```

## Script para Linux

### Uso básico

**Instalar and Actualizar**

```shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) install
```
El puerto de escucha predeterminado es `9000` y el puerto de Desafío HTTP predeterminado es `9180`.
Si hay un conflicto de puertos, modifique manualmente `/usr/local/etc/nginx-ui/app.ini`,
luego use `systemctl restart nginx-ui` para recargar el servicio de UI de Nginx.

**Eliminar UI Nginx UI, excepto los archivos de configuración y la base de datos**

```shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) remove
```

### Uso avanzado

````shell
bash <(curl -L -s https://raw.githubusercontent.com/0xJacky/nginx-ui/master/install.sh) help
````

## Ejemplo de configuración de proxy reverso de Nginx

```nginx
server {
    listen          80;
    listen          [::]:80;

    server_name     <your_server_name>;
    rewrite ^(.*)$  https://$host$1 permanent;
}

map $http_upgrade $connection_upgrade {
    default upgrade;
    ''      close;
}

server {
    listen  443       ssl;
    listen  [::]:443  ssl;
    http2   on;

    server_name         <your_server_name>;

    ssl_certificate     /path/to/ssl_cert;
    ssl_certificate_key /path/to/ssl_cert_key;

    location / {
        proxy_set_header    Host                $host;
        proxy_set_header    X-Real-IP           $remote_addr;
        proxy_set_header    X-Forwarded-For     $proxy_add_x_forwarded_for;
        proxy_set_header    X-Forwarded-Proto   $scheme;
        proxy_http_version  1.1;
        proxy_set_header    Upgrade             $http_upgrade;
        proxy_set_header    Connection          $connection_upgrade;
        proxy_pass          http://127.0.0.1:9000/;
    }
}
```

## Contribuir

Las contribuciones son lo que hace que la comunidad de código abierto sea un lugar increíble para aprender, inspirar y crear. Cualquier contribución que hagas es **muy apreciada**.

Si tiene una sugerencia que mejoraría este proyecto, bifurque el repositorio y cree un pull request. También puede simplemente abrir un issue con la etiqueta "enhancement". ¡No olvides darle una estrella al proyecto! ¡Gracias de nuevo!

1. Bifurcar el proyecto
2. Crea un branch de mejora (`git checkout -b feature/AmazingFeature`)
3. Commit de tus cambios (`git commit -m 'Add some AmazingFeature'`)
4. Push al branch (`git push origin feature/AmazingFeature`)
5. Abrir un Pull Request

## Licencia

Este proyecto se proporciona bajo una licencia GNU Affero General Public License v3.0 que se puede encontrar en el archivo [LICENCIA](LICENSE). Al usar, distribuir o contribuir a este proyecto, acepta los términos y condiciones de esta licencia.
