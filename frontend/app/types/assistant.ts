export type AssistantMessageStatus = 'running' | 'completed' | 'failed' | 'cancelled' | 'timed_out' | 'interrupted'

export interface AssistantConversation {
  id: string
  title: string
  createdAt: string
  updatedAt: string
}

export interface AssistantMessage {
  id: string
  conversationId: string
  role: 'user' | 'assistant'
  content: string
  status: AssistantMessageStatus
  createdAt: string
}

export interface AssistantEvent {
  type: 'run_started' | 'text_delta' | 'tool_started' | 'tool_finished' | 'run_completed' | 'run_failed'
  runId: string
  conversationId: string
  delta?: string
  toolCallId?: string
  toolName?: string
  message?: AssistantMessage
  error?: string
}

export interface AgentMessage {
  id: string
  role: 'user' | 'assistant'
  parts: { type: 'text', text: string }[]
  metadata: { status: AssistantMessageStatus }
}
