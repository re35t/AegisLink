import { AegisLinkServer } from "./aegislink-server.js";
import { createAegisLinkHttpServer } from "./http.js";

const port = Number.parseInt(process.env.PORT ?? "4321", 10);
const host = process.env.HOST ?? "127.0.0.1";
const service = new AegisLinkServer();
const server = createAegisLinkHttpServer({ service });

server.on("error", (error: NodeJS.ErrnoException) => {
  if (error.code === "EADDRINUSE") {
    console.error(`Port ${port} is already in use. Try PORT=4322 pnpm run dev:server.`);
  } else if (error.code === "EPERM") {
    console.error(`Cannot bind ${host}:${port}. Check local sandbox/firewall permissions or try another PORT/HOST.`);
  } else {
    console.error(error);
  }
  process.exitCode = 1;
});

server.listen(port, host, () => {
  console.log(`AegisLink server listening on http://${host}:${port}`);
});
