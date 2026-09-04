export interface APIErrorResponse {
  error: {
    code: string
    message: string
    fields: Record<string, string>
    request_id: string
  }
}

export interface LoginResponse {
  authenticated: true
  username: string
}

export interface SessionResponse extends LoginResponse {
  expires_at: string
}
