services:
  b-ui:
    image: ${IMAGE_REF}
    container_name: ${CONTAINER_NAME}
    hostname: ${CONTAINER_NAME}
    environment:
      PANEL_PORT: "${PANEL_PORT}"
      PANEL_PATH: "${PANEL_PATH}"
      SUB_PORT: "${SUB_PORT}"
      SUB_PATH: "${SUB_PATH}"
    volumes:
      - ./db:/app/db
      - ./cert:/app/cert
    tty: true
    restart: unless-stopped
    ports:
${PORT_LINES}
    entrypoint: ./entrypoint.sh
