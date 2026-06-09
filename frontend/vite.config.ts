import { defineConfig, loadEnv } from 'vite'
import { resolve } from 'path'

const libs = [
    resolve(__dirname, 'src/modules/polyfill/url-pattern-polyfill.ts'),
]

const config = {
    entry: [resolve(__dirname, 'src/sonary.ts'), ...libs],
}

export default ({ mode }: { mode: string }) => {
    const { VITE_HOST, VITE_PORT, VITE_BASE_URL, NODE_ENV } = loadEnv(mode, process.cwd(), '')

    return defineConfig({
        base: VITE_BASE_URL,
        plugins: [],
        define: {
            'process.env.NODE_ENV': JSON.stringify(NODE_ENV)
        },
        resolve: {
            alias: {
                '@': resolve(__dirname, 'src'),
            }
        },
        optimizeDeps: {
            exclude: [],
        },
        server: {
            port: Number(VITE_PORT),
            host: VITE_HOST,
            open: false,
            cors: true,
            proxy: {},
            hmr: {
                host: VITE_HOST,
                port: 3102,
            },
            watch: {
                ignored: ["**/public/assets/**"],
            },
        },
        // @ts-ignore
        build: Object.assign(config ? {
            lib: {
                ...config,
                formats: ['es'],
            },
            emptyOutDir: true,
            manifest: false,
            cssCodeSplit: false,
            minify: 'terser'
        } : {
            manifest: true,
            chunkSizeWarningLimit: 2000,
            rollupOptions: {
                input: {
                    app: './index.html',
                },
                output: {
                    entryFileNames: 'static/js/[name]-[hash].js',
                    chunkFileNames: 'static/js/[name]-[hash].js',
                    assetFileNames: 'static/[ext]/[name]-[hash].[ext]',
                    compact: true,
                },
            },
        }, {
            sourcemap: false,
            target: 'es2023',
            outDir: '../static/build',
            reportCompressedSize: false,
        })
    })
}
