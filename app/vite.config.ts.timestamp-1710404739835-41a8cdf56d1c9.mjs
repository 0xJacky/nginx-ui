// vite.config.ts
import { URL, fileURLToPath } from "node:url";
import { defineConfig, loadEnv } from "file:///Users/Jacky/Sites/nginx-ui/app/node_modules/.pnpm/vite@5.1.1_@types+node@20.10.2_less@4.2.0/node_modules/vite/dist/node/index.js";
import vue from "file:///Users/Jacky/Sites/nginx-ui/app/node_modules/.pnpm/@vitejs+plugin-vue@5.0.3_vite@5.1.1_vue@3.4.17/node_modules/@vitejs/plugin-vue/dist/index.mjs";
import Components from "file:///Users/Jacky/Sites/nginx-ui/app/node_modules/.pnpm/unplugin-vue-components@0.26.0_vue@3.4.17/node_modules/unplugin-vue-components/dist/vite.js";
import { AntDesignVueResolver } from "file:///Users/Jacky/Sites/nginx-ui/app/node_modules/.pnpm/unplugin-vue-components@0.26.0_vue@3.4.17/node_modules/unplugin-vue-components/dist/resolvers.js";
import vueJsx from "file:///Users/Jacky/Sites/nginx-ui/app/node_modules/.pnpm/@vitejs+plugin-vue-jsx@3.1.0_vite@5.1.1_vue@3.4.17/node_modules/@vitejs/plugin-vue-jsx/dist/index.mjs";
import vitePluginBuildId from "file:///Users/Jacky/Sites/nginx-ui/app/node_modules/.pnpm/vite-plugin-build-id@0.2.8_less@4.2.0/node_modules/vite-plugin-build-id/dist/index.js";
import svgLoader from "file:///Users/Jacky/Sites/nginx-ui/app/node_modules/.pnpm/vite-svg-loader@5.1.0_vue@3.4.17/node_modules/vite-svg-loader/index.js";
import AutoImport from "file:///Users/Jacky/Sites/nginx-ui/app/node_modules/.pnpm/unplugin-auto-import@0.17.3_@vueuse+core@10.7.2/node_modules/unplugin-auto-import/dist/vite.js";
import DefineOptions from "file:///Users/Jacky/Sites/nginx-ui/app/node_modules/.pnpm/unplugin-vue-define-options@1.4.1_vue@3.4.17/node_modules/unplugin-vue-define-options/dist/vite.mjs";
var __vite_injected_original_import_meta_url = "file:///Users/Jacky/Sites/nginx-ui/app/vite.config.ts";
var vite_config_default = defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), "");
  return {
    base: "./",
    resolve: {
      alias: {
        "@": fileURLToPath(new URL("./src", __vite_injected_original_import_meta_url))
      },
      extensions: [
        ".mjs",
        ".js",
        ".ts",
        ".jsx",
        ".tsx",
        ".json",
        ".vue",
        ".less"
      ]
    },
    plugins: [
      vue(),
      vueJsx(),
      vitePluginBuildId(),
      svgLoader(),
      Components({
        resolvers: [AntDesignVueResolver({ importStyle: false })],
        directoryAsNamespace: true
      }),
      AutoImport({
        imports: ["vue", "vue-router", "pinia"],
        vueTemplate: true
      }),
      DefineOptions()
    ],
    css: {
      preprocessorOptions: {
        less: {
          modifyVars: {
            "border-radius-base": "5px"
          },
          javascriptEnabled: true
        }
      }
    },
    server: {
      proxy: {
        "/api": {
          target: env.VITE_PROXY_TARGET || "http://localhost:9000",
          changeOrigin: true,
          secure: false,
          ws: true
        }
      }
    },
    build: {
      chunkSizeWarningLimit: 1e3
    }
  };
});
export {
  vite_config_default as default
};
//# sourceMappingURL=data:application/json;base64,ewogICJ2ZXJzaW9uIjogMywKICAic291cmNlcyI6IFsidml0ZS5jb25maWcudHMiXSwKICAic291cmNlc0NvbnRlbnQiOiBbImNvbnN0IF9fdml0ZV9pbmplY3RlZF9vcmlnaW5hbF9kaXJuYW1lID0gXCIvVXNlcnMvSmFja3kvU2l0ZXMvbmdpbngtdWkvYXBwXCI7Y29uc3QgX192aXRlX2luamVjdGVkX29yaWdpbmFsX2ZpbGVuYW1lID0gXCIvVXNlcnMvSmFja3kvU2l0ZXMvbmdpbngtdWkvYXBwL3ZpdGUuY29uZmlnLnRzXCI7Y29uc3QgX192aXRlX2luamVjdGVkX29yaWdpbmFsX2ltcG9ydF9tZXRhX3VybCA9IFwiZmlsZTovLy9Vc2Vycy9KYWNreS9TaXRlcy9uZ2lueC11aS9hcHAvdml0ZS5jb25maWcudHNcIjtpbXBvcnQgeyBVUkwsIGZpbGVVUkxUb1BhdGggfSBmcm9tICdub2RlOnVybCdcbmltcG9ydCB7IGRlZmluZUNvbmZpZywgbG9hZEVudiB9IGZyb20gJ3ZpdGUnXG5pbXBvcnQgdnVlIGZyb20gJ0B2aXRlanMvcGx1Z2luLXZ1ZSdcbmltcG9ydCBDb21wb25lbnRzIGZyb20gJ3VucGx1Z2luLXZ1ZS1jb21wb25lbnRzL3ZpdGUnXG5pbXBvcnQgeyBBbnREZXNpZ25WdWVSZXNvbHZlciB9IGZyb20gJ3VucGx1Z2luLXZ1ZS1jb21wb25lbnRzL3Jlc29sdmVycydcbmltcG9ydCB2dWVKc3ggZnJvbSAnQHZpdGVqcy9wbHVnaW4tdnVlLWpzeCdcblxuaW1wb3J0IHZpdGVQbHVnaW5CdWlsZElkIGZyb20gJ3ZpdGUtcGx1Z2luLWJ1aWxkLWlkJ1xuaW1wb3J0IHN2Z0xvYWRlciBmcm9tICd2aXRlLXN2Zy1sb2FkZXInXG5pbXBvcnQgQXV0b0ltcG9ydCBmcm9tICd1bnBsdWdpbi1hdXRvLWltcG9ydC92aXRlJ1xuaW1wb3J0IERlZmluZU9wdGlvbnMgZnJvbSAndW5wbHVnaW4tdnVlLWRlZmluZS1vcHRpb25zL3ZpdGUnXG5cbi8vIGh0dHBzOi8vdml0ZWpzLmRldi9jb25maWcvXG5leHBvcnQgZGVmYXVsdCBkZWZpbmVDb25maWcoKHsgbW9kZSB9KSA9PiB7XG4gIC8vIGVzbGludC1kaXNhYmxlLW5leHQtbGluZSBuL3ByZWZlci1nbG9iYWwvcHJvY2Vzc1xuICBjb25zdCBlbnYgPSBsb2FkRW52KG1vZGUsIHByb2Nlc3MuY3dkKCksICcnKVxuXG4gIHJldHVybiB7XG4gICAgYmFzZTogJy4vJyxcbiAgICByZXNvbHZlOiB7XG4gICAgICBhbGlhczoge1xuICAgICAgICAnQCc6IGZpbGVVUkxUb1BhdGgobmV3IFVSTCgnLi9zcmMnLCBpbXBvcnQubWV0YS51cmwpKSxcbiAgICAgIH0sXG4gICAgICBleHRlbnNpb25zOiBbXG4gICAgICAgICcubWpzJyxcbiAgICAgICAgJy5qcycsXG4gICAgICAgICcudHMnLFxuICAgICAgICAnLmpzeCcsXG4gICAgICAgICcudHN4JyxcbiAgICAgICAgJy5qc29uJyxcbiAgICAgICAgJy52dWUnLFxuICAgICAgICAnLmxlc3MnLFxuICAgICAgXSxcbiAgICB9LFxuICAgIHBsdWdpbnM6IFtcbiAgICAgIHZ1ZSgpLFxuICAgICAgdnVlSnN4KCksXG5cbiAgICAgIHZpdGVQbHVnaW5CdWlsZElkKCksXG4gICAgICBzdmdMb2FkZXIoKSxcbiAgICAgIENvbXBvbmVudHMoe1xuICAgICAgICByZXNvbHZlcnM6IFtBbnREZXNpZ25WdWVSZXNvbHZlcih7IGltcG9ydFN0eWxlOiBmYWxzZSB9KV0sXG4gICAgICAgIGRpcmVjdG9yeUFzTmFtZXNwYWNlOiB0cnVlLFxuICAgICAgfSksXG4gICAgICBBdXRvSW1wb3J0KHtcbiAgICAgICAgaW1wb3J0czogWyd2dWUnLCAndnVlLXJvdXRlcicsICdwaW5pYSddLFxuICAgICAgICB2dWVUZW1wbGF0ZTogdHJ1ZSxcbiAgICAgIH0pLFxuICAgICAgRGVmaW5lT3B0aW9ucygpLFxuICAgIF0sXG4gICAgY3NzOiB7XG4gICAgICBwcmVwcm9jZXNzb3JPcHRpb25zOiB7XG4gICAgICAgIGxlc3M6IHtcbiAgICAgICAgICBtb2RpZnlWYXJzOiB7XG4gICAgICAgICAgICAnYm9yZGVyLXJhZGl1cy1iYXNlJzogJzVweCcsXG4gICAgICAgICAgfSxcbiAgICAgICAgICBqYXZhc2NyaXB0RW5hYmxlZDogdHJ1ZSxcbiAgICAgICAgfSxcbiAgICAgIH0sXG4gICAgfSxcbiAgICBzZXJ2ZXI6IHtcbiAgICAgIHByb3h5OiB7XG4gICAgICAgICcvYXBpJzoge1xuICAgICAgICAgIHRhcmdldDogZW52LlZJVEVfUFJPWFlfVEFSR0VUIHx8ICdodHRwOi8vbG9jYWxob3N0OjkwMDAnLFxuICAgICAgICAgIGNoYW5nZU9yaWdpbjogdHJ1ZSxcbiAgICAgICAgICBzZWN1cmU6IGZhbHNlLFxuICAgICAgICAgIHdzOiB0cnVlLFxuICAgICAgICB9LFxuICAgICAgfSxcbiAgICB9LFxuICAgIGJ1aWxkOiB7XG4gICAgICBjaHVua1NpemVXYXJuaW5nTGltaXQ6IDEwMDAsXG4gICAgfSxcbiAgfVxufSlcbiJdLAogICJtYXBwaW5ncyI6ICI7QUFBK1EsU0FBUyxLQUFLLHFCQUFxQjtBQUNsVCxTQUFTLGNBQWMsZUFBZTtBQUN0QyxPQUFPLFNBQVM7QUFDaEIsT0FBTyxnQkFBZ0I7QUFDdkIsU0FBUyw0QkFBNEI7QUFDckMsT0FBTyxZQUFZO0FBRW5CLE9BQU8sdUJBQXVCO0FBQzlCLE9BQU8sZUFBZTtBQUN0QixPQUFPLGdCQUFnQjtBQUN2QixPQUFPLG1CQUFtQjtBQVY0SSxJQUFNLDJDQUEyQztBQWF2TixJQUFPLHNCQUFRLGFBQWEsQ0FBQyxFQUFFLEtBQUssTUFBTTtBQUV4QyxRQUFNLE1BQU0sUUFBUSxNQUFNLFFBQVEsSUFBSSxHQUFHLEVBQUU7QUFFM0MsU0FBTztBQUFBLElBQ0wsTUFBTTtBQUFBLElBQ04sU0FBUztBQUFBLE1BQ1AsT0FBTztBQUFBLFFBQ0wsS0FBSyxjQUFjLElBQUksSUFBSSxTQUFTLHdDQUFlLENBQUM7QUFBQSxNQUN0RDtBQUFBLE1BQ0EsWUFBWTtBQUFBLFFBQ1Y7QUFBQSxRQUNBO0FBQUEsUUFDQTtBQUFBLFFBQ0E7QUFBQSxRQUNBO0FBQUEsUUFDQTtBQUFBLFFBQ0E7QUFBQSxRQUNBO0FBQUEsTUFDRjtBQUFBLElBQ0Y7QUFBQSxJQUNBLFNBQVM7QUFBQSxNQUNQLElBQUk7QUFBQSxNQUNKLE9BQU87QUFBQSxNQUVQLGtCQUFrQjtBQUFBLE1BQ2xCLFVBQVU7QUFBQSxNQUNWLFdBQVc7QUFBQSxRQUNULFdBQVcsQ0FBQyxxQkFBcUIsRUFBRSxhQUFhLE1BQU0sQ0FBQyxDQUFDO0FBQUEsUUFDeEQsc0JBQXNCO0FBQUEsTUFDeEIsQ0FBQztBQUFBLE1BQ0QsV0FBVztBQUFBLFFBQ1QsU0FBUyxDQUFDLE9BQU8sY0FBYyxPQUFPO0FBQUEsUUFDdEMsYUFBYTtBQUFBLE1BQ2YsQ0FBQztBQUFBLE1BQ0QsY0FBYztBQUFBLElBQ2hCO0FBQUEsSUFDQSxLQUFLO0FBQUEsTUFDSCxxQkFBcUI7QUFBQSxRQUNuQixNQUFNO0FBQUEsVUFDSixZQUFZO0FBQUEsWUFDVixzQkFBc0I7QUFBQSxVQUN4QjtBQUFBLFVBQ0EsbUJBQW1CO0FBQUEsUUFDckI7QUFBQSxNQUNGO0FBQUEsSUFDRjtBQUFBLElBQ0EsUUFBUTtBQUFBLE1BQ04sT0FBTztBQUFBLFFBQ0wsUUFBUTtBQUFBLFVBQ04sUUFBUSxJQUFJLHFCQUFxQjtBQUFBLFVBQ2pDLGNBQWM7QUFBQSxVQUNkLFFBQVE7QUFBQSxVQUNSLElBQUk7QUFBQSxRQUNOO0FBQUEsTUFDRjtBQUFBLElBQ0Y7QUFBQSxJQUNBLE9BQU87QUFBQSxNQUNMLHVCQUF1QjtBQUFBLElBQ3pCO0FBQUEsRUFDRjtBQUNGLENBQUM7IiwKICAibmFtZXMiOiBbXQp9Cg==
