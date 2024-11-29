// @ts-check
import { defineConfig } from "astro/config";
import react from "@astrojs/react";
import tailwind from "@astrojs/tailwind";
import { fileURLToPath } from 'url';
import path from 'path';

// Get the folder name dynamically
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const folderName = path.basename(__dirname);

// https://astro.build/config
export default defineConfig({
  base: `/custom-developments/${folderName}`,
  integrations: [
    react(),
    tailwind({
      applyBaseStyles: false,
    }),
  ],
});
