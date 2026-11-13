export type Health = {
  ok: boolean
  whatsapp: {
    mode: "off" | "test" | "live" | ""
    linked: boolean
    connected: boolean
  }
}

export type Person = {
  id: number
  name: string
  phone: string
  isAdmin: boolean
}

export type PersonInput = {
  name: string
  phone: string
}

export type Service = {
  id: number
  name: string
  weekday: number
  startTime: string
  active: boolean
}

export type ServiceInput = Omit<Service, "id">

export type RotationUnit = "week" | "service"

export type Ministry = {
  id: number
  name: string
  rotationUnit: RotationUnit
  memberCount: number
}

export type MinistryInput = {
  name: string
  rotationUnit: RotationUnit
}

export type Role = {
  id: number
  name: string
  slots: number
}

export type RoleInput = Omit<Role, "id">

export type MemberRole = {
  roleId: number
  fixed: boolean
}

export type Member = {
  personId: number
  name: string
  phone: string
  isLeader: boolean
  roles: MemberRole[]
}

export type MemberInput = {
  isLeader: boolean
  roles: MemberRole[]
}

export type MinistryDetail = Ministry & {
  roles: Role[]
  members: Member[]
  serviceIds: number[]
}

export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.status = status
  }
}

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const response = await fetch(`/api${path}`, {
    method,
    headers: body === undefined ? undefined : { "Content-Type": "application/json" },
    body: body === undefined ? undefined : JSON.stringify(body),
  })
  if (!response.ok) {
    const data = (await response.json().catch(() => null)) as { error?: string } | null
    throw new ApiError(response.status, data?.error ?? `Erro ${response.status}`)
  }
  if (response.status === 204) {
    return undefined as T
  }
  return (await response.json()) as T
}

export const api = {
  health: () => request<Health>("GET", "/health"),
  me: () => request<Person>("GET", "/me"),
  redeem: (token: string) => request<Person>("POST", "/auth/redeem", { token }),
  logout: () => request<void>("POST", "/auth/logout"),

  people: {
    list: () => request<Person[]>("GET", "/people"),
    create: (input: PersonInput) => request<Person>("POST", "/people", input),
    update: (id: number, input: PersonInput) => request<Person>("PUT", `/people/${id}`, input),
    archive: (id: number) => request<void>("DELETE", `/people/${id}`),
  },

  services: {
    list: () => request<Service[]>("GET", "/services"),
    create: (input: ServiceInput) => request<Service>("POST", "/services", input),
    update: (id: number, input: ServiceInput) => request<Service>("PUT", `/services/${id}`, input),
  },

  ministries: {
    list: () => request<Ministry[]>("GET", "/ministries"),
    get: (id: number) => request<MinistryDetail>("GET", `/ministries/${id}`),
    create: (input: MinistryInput) => request<Ministry>("POST", "/ministries", input),
    update: (id: number, input: MinistryInput) => request<void>("PUT", `/ministries/${id}`, input),
    archive: (id: number) => request<void>("DELETE", `/ministries/${id}`),
    createRole: (id: number, input: RoleInput) => request<Role>("POST", `/ministries/${id}/roles`, input),
    updateRole: (id: number, roleId: number, input: RoleInput) =>
      request<void>("PUT", `/ministries/${id}/roles/${roleId}`, input),
    deleteRole: (id: number, roleId: number) => request<void>("DELETE", `/ministries/${id}/roles/${roleId}`),
    putMember: (id: number, personId: number, input: MemberInput) =>
      request<void>("PUT", `/ministries/${id}/members/${personId}`, input),
    removeMember: (id: number, personId: number) =>
      request<void>("DELETE", `/ministries/${id}/members/${personId}`),
    setServices: (id: number, serviceIds: number[]) =>
      request<void>("PUT", `/ministries/${id}/services`, { serviceIds }),
  },
}

export const queryKeys = {
  health: ["health"] as const,
  me: ["me"] as const,
  people: ["people"] as const,
  services: ["services"] as const,
  ministries: ["ministries"] as const,
  ministry: (id: number) => ["ministries", id] as const,
}
