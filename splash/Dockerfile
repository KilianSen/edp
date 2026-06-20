# edp-splash: a tiny static site served by nginx.
FROM nginx:1.27-alpine

COPY nginx.conf /etc/nginx/conf.d/default.conf
COPY index.html styles.css app.js config.js /usr/share/nginx/html/

EXPOSE 80
