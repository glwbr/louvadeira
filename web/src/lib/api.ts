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
  communionWeek: number | null
}

export type ServiceInput = Omit<Service, "id">

export type RotationUnit = "week" | "service"

export type Ministry = {
  id: number
  name: string
  rotationUnit: RotationUnit
  whatsappGroup: string
  memberCount: number
}

export type MinistryInput = {
  name: string
  rotationUnit: RotationUnit
  whatsappGroup: string
}

export type WhatsAppGroup = {
  id: string
  name: string
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

export type SlotWarning = "empty" | "unavailable" | "doubleBooked" | "notMember"

export type AssignmentStatus = "scheduled" | "confirmed" | "declined"

export type ScheduleSlot = {
  assignmentId: number
  personId: number
  personName: string
  status: AssignmentStatus
  coverOpen: boolean
  warning?: SlotWarning
}

export type CoverRequest = {
  id: number
  assignmentId: number
  date: string
  serviceName: string
  startTime: string
  roleName: string
  absentName: string
  status: "open" | "covered" | "uncovered"
  late: boolean
  coveredBy: string
  createdAt: string
  invites: { personId: number; name: string; answer: "" | "yes" | "no" }[]
}

export type ScheduleRole = {
  roleId: number
  name: string
  slots: ScheduleSlot[]
}

export type ScheduleService = {
  serviceId: number
  date: string
  name: string
  startTime: string
  communion: boolean
  roles: ScheduleRole[]
}

export type ScheduleWeek = {
  start: string
  services: ScheduleService[]
}

export type ScheduleView = {
  id: number
  ministryId: number
  month: string
  status: "draft" | "published"
  generatedAt: string
  publishedAt: string
  warnings: number
  weeks: ScheduleWeek[]
}

export type Reason = "viagem" | "trabalho" | "saude" | "familia" | "estudos" | "outro"

export type Repeat = "none" | "weekly" | "monthly"

export type Unavailability = {
  id: number
  personId: number
  personName: string
  startDate: string
  /** NOTE: Empty when a repeating absence has no end. */
  endDate: string
  reason: Reason
  repeat: Repeat
  weekday: number | null
  weekOfMonth: number | null
  /** NOTE: Services the person still holds in published schedules during the absence, swaps not opened yet. */
  scheduled: number
}

export type UnavailabilityInput = Omit<Unavailability, "id" | "personName" | "scheduled">

export type MemberSlot = {
  assignmentId: number
  ministryId: number
  ministryName: string
  serviceId: number
  serviceName: string
  date: string
  startTime: string
  roleName: string
  status: AssignmentStatus
  coverOpen: boolean
  communion: boolean
}

export type AffectedSlot = {
  assignmentId: number
  ministryId: number
  ministryName: string
  date: string
  startTime: string
  serviceName: string
  roleName: string
  status: AssignmentStatus
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
  requestLink: (phone: string) => request<{ ok: boolean }>("POST", "/auth/link", { phone }),

  mine: {
    slots: () => request<MemberSlot[]>("GET", "/me/assignments"),
    confirm: (assignmentIds: number[]) => request<void>("POST", "/me/confirm", { assignmentIds }),
    decline: (assignmentIds: number[]) => request<CoverRequest[]>("POST", "/me/decline", { assignmentIds }),
    absences: () => request<Unavailability[]>("GET", "/me/unavailabilities"),
    createAbsence: (input: Omit<UnavailabilityInput, "personId">) =>
      request<Unavailability>("POST", "/me/unavailabilities", input),
    removeAbsence: (id: number) => request<void>("DELETE", `/me/unavailabilities/${id}`),
    absenceSlots: (id: number) => request<AffectedSlot[]>("GET", `/me/unavailabilities/${id}/assignments`),
    openCovers: (id: number) => request<CoverRequest[]>("POST", `/me/unavailabilities/${id}/covers`),
  },

  people: {
    list: () => request<Person[]>("GET", "/people"),
    create: (input: PersonInput) => request<Person>("POST", "/people", input),
    update: (id: number, input: PersonInput) => request<Person>("PUT", `/people/${id}`, input),
    archive: (id: number) => request<void>("DELETE", `/people/${id}`),
    sendLink: (id: number) => request<void>("POST", `/people/${id}/link`),
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
    reorderRoles: (id: number, roleIds: number[]) => request<void>("PUT", `/ministries/${id}/roles/order`, { roleIds }),
  },

  schedules: {
    get: (ministryId: number, month: string) =>
      request<ScheduleView>("GET", `/ministries/${ministryId}/schedules/${month}`),
    generate: (ministryId: number, month: string) =>
      request<ScheduleView>("POST", `/ministries/${ministryId}/schedules/${month}/generate`),
    discard: (ministryId: number, month: string) =>
      request<void>("DELETE", `/ministries/${ministryId}/schedules/${month}`),
    publish: (ministryId: number, month: string) =>
      request<void>("POST", `/ministries/${ministryId}/schedules/${month}/publish`),
    assign: (ministryId: number, month: string, assignmentId: number, personId: number | null) =>
      request<void>("PUT", `/ministries/${ministryId}/schedules/${month}/assignments/${assignmentId}`, { personId }),
    decline: (ministryId: number, month: string, assignmentId: number) =>
      request<CoverRequest>("POST", `/ministries/${ministryId}/schedules/${month}/assignments/${assignmentId}/decline`),
    covers: (ministryId: number, month: string) =>
      request<CoverRequest[]>("GET", `/ministries/${ministryId}/schedules/${month}/covers`),
  },

  whatsapp: {
    groups: () => request<WhatsAppGroup[]>("GET", "/admin/whatsapp/groups"),
  },

  unavailabilities: {
    list: (from: string, to: string) => request<Unavailability[]>("GET", `/unavailabilities?from=${from}&to=${to}`),
    create: (input: UnavailabilityInput) => request<Unavailability>("POST", "/unavailabilities", input),
    remove: (id: number) => request<void>("DELETE", `/unavailabilities/${id}`),
    assignments: (id: number) => request<AffectedSlot[]>("GET", `/unavailabilities/${id}/assignments`),
    openCovers: (id: number) => request<CoverRequest[]>("POST", `/unavailabilities/${id}/covers`),
  },
}

export const queryKeys = {
  health: ["health"] as const,
  me: ["me"] as const,
  people: ["people"] as const,
  services: ["services"] as const,
  ministries: ["ministries"] as const,
  ministry: (id: number) => ["ministries", id] as const,
  schedule: (ministryId: number, month: string) => ["schedules", ministryId, month] as const,
  covers: (ministryId: number, month: string) => ["schedules", ministryId, month, "covers"] as const,
  whatsappGroups: ["whatsapp-groups"] as const,
  unavailabilities: (from: string, to: string) => ["unavailabilities", from, to] as const,
  unavailabilityAssignments: (id: number) => ["unavailabilities", id, "assignments"] as const,
  mySlots: ["mine", "slots"] as const,
  myAbsences: ["mine", "absences"] as const,
  myAbsenceSlots: (id: number) => ["mine", "absences", id, "slots"] as const,
}
