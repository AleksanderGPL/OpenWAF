export interface Service {
  id: number
  name: string
  hostname: string
  upstreamUrl: string
  skipTlsVerify: boolean
  enabled: boolean
  createdAt: string
  updatedAt: string
}

export interface ServiceInput {
  name: string
  hostname: string
  upstreamUrl: string
  skipTlsVerify: boolean
  enabled: boolean
}
