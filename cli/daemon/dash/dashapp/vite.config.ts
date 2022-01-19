import reactRefresh from '@vitejs/plugin-react-refresh'
import path from 'path';
import {defineConfig} from 'vite'

const projectRootDir = path.resolve(__dirname)

// https://vitejs.dev/config/
export default defineConfig({
  plugins: [reactRefresh()],
  alias: [
    {find: "~c", replacement: path.resolve(projectRootDir, "src", "components")},
    {find: "~p", replacement: path.resolve(projectRootDir, "src", "pages")},
    {find: "~mod", replacement: path.resolve(projectRootDir, "src", "mod")},
    {find: "~lib", replacement: path.resolve(projectRootDir, "src", "lib")},
  ],
})
