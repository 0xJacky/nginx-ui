import type { FrontendTask } from '../types'
import HttpsCheckTask from './https-check'
import WebsocketTask from './websocket'

// Collection of all frontend tasks
const frontendTasks: Record<string, FrontendTask> = {
  'Frontend-Websocket': WebsocketTask,
  'Frontend-HttpsCheck': HttpsCheckTask,
  // Add more frontend tasks here
}

export default frontendTasks
