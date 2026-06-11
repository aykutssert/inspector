import fs from "fs";
import { pipeline } from "stream";

declare const res: any;

// --- positive: inline createReadStream piped with no error handler ---
export function download(path: string) {
  fs.createReadStream(path).pipe(res); // FIRE
}

// --- negative: error handler attached to the source ---
export function downloadGuarded(path: string) {
  fs.createReadStream(path)
    .on("error", () => res.status(404).end())
    .pipe(res); // NO FIRE
}

// --- negative: pipeline() handles errors ---
export function downloadPipeline(path: string) {
  pipeline(fs.createReadStream(path), res, (err) => {
    if (err) res.status(500).end();
  }); // NO FIRE
}
