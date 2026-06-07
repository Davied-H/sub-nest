import { Fragment, useCallback, useEffect, useMemo, useState, type ComponentProps, type FormEvent } from "react"
import {
  ActivityIcon,
  ArchiveRestoreIcon,
  BugIcon,
  CheckCircle2Icon,
  ClipboardIcon,
  DatabaseBackupIcon,
  DownloadIcon,
  EyeIcon,
  EyeOffIcon,
  FileJsonIcon,
  FileTextIcon,
  KeyRoundIcon,
  Loader2Icon,
  LockIcon,
  PauseCircleIcon,
  PlusIcon,
  RefreshCcwIcon,
  SearchIcon,
  SaveIcon,
  ServerIcon,
  UserIcon,
  ShieldIcon,
  Trash2Icon,
  WifiOffIcon,
} from "lucide-react"
import { FiChevronDown, FiChevronRight } from "react-icons/fi"
import { toast } from "sonner"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  Empty,
  EmptyContent,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import {
  Field,
  FieldDescription,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Skeleton } from "@/components/ui/skeleton"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { Textarea } from "@/components/ui/textarea"
import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

type Dashboard = {
  sourceCount: number
  enabledSources: number
  healthySources: number
  unhealthySources: number
  outputCount: number
  enabledOutputs: number
  totalCachedNodes: number
  lastRefreshAt?: string
  publicExampleUrl: string
  needsAdminSetup: boolean
}

type Source = {
  id: string
  name: string
  url?: string
  sourceType?: "url" | "file"
  fileName?: string
  fileContent?: string
  trafficQuery: TrafficQuery
  trafficInfo: TrafficInfo
  urlMasked: string
  enabled: boolean
  remark: string
  tags: string[]
  lastStatus: string
  lastFormat: string
  lastNodeCount: number
  lastError: string
  refreshProgress: string
  refreshPercent: number
  lastRefreshedAt?: string
  lastSuccessAt?: string
  nodeStats: {
    total: number
    alive: number
    dead: number
    unchecked: number
  }
  nodes?: Array<{
    key: string
    name: string
    originalName: string
    sourceId: string
    source: string
    server: string
    port: number
    region: string
    regionCode: string
    resolvedIp: string
    exitIp: string
    delayMs: number
    alive?: boolean
    excludedReason: string
    regionSource: string
    probeStatus: string
    probeError: string
  }>
}

type TrafficQuery = {
  mode: "disabled" | "subscription-header" | "subscription-body-regex" | "custom-http"
  url?: string
  method?: string
  headers?: Record<string, string>
  body?: string
  parser?: {
    type?: "json-path" | "regex" | "subscription-header"
    upload?: string
    download?: string
    total?: string
    remaining?: string
    expire?: string
  }
}

type TrafficInfo = {
  uploadBytes: number
  downloadBytes: number
  totalBytes: number
  remainingBytes: number
  expireAt?: string
  lastCheckedAt?: string
  lastStatus: string
  lastError: string
  debug?: TrafficDebug
}

type TrafficDebug = {
  method?: string
  url?: string
  status?: string
  statusCode?: number
  contentType?: string
  parserType?: string
  bodyPreview?: string
  header?: string
  paths?: Array<{
    label: string
    path: string
    found: boolean
    value?: string
    error?: string
  }>
}

type Output = {
  id: string
  slug: string
  name: string
  enabled: boolean
  format: string
  sourceIds: string[]
  filter: {
    includeKeywords: string[]
    excludeKeywords: string[]
    regex: string
  }
  renameRules: Array<{ pattern: string; replacement: string }>
  nodeNameOverrides?: Record<string, string>
  pac: {
    enabled: boolean
    enabledSet?: boolean
    proxy: string
    ruleSetId?: string
    ruleSourceUrl: string
    ruleSourceFormat: string
    ruleRefreshHours: number
    domainKeywords?: string[]
    directDomainSuffixes: string[]
    directCidrs: string[]
    cachedDomainSuffixes?: string[]
    lastSyncedAt?: string
    lastSyncStatus?: string
    lastSyncError?: string
  }
  groupMode: string
  lastGeneratedAt?: string
  lastNodeCount: number
  lastDroppedCount: number
  nodeNames?: string[]
}

type RuleSource = {
  id: string
  name: string
  url: string
  format: string
  refreshHours: number
  localPath?: string
  cachedDomainSuffixes?: string[]
  cachedDomainCount?: number
  lastSyncedAt?: string
  lastSyncStatus?: string
  lastSyncError?: string
}

type RuleSet = {
  id: string
  name: string
  sourceIds: string[]
  domainKeywords?: string[]
  directDomainSuffixes: string[]
  excludedDomainSuffixes?: string[]
  directCidrs: string[]
  cachedDomainSuffixes?: string[]
  cachedDomainCount?: number
  lastSyncedAt?: string
  lastSyncStatus?: string
  lastSyncError?: string
}

type RuleDomain = {
  domain: string
  source: string
  type: "cache" | "manual" | "keyword" | "excluded" | string
}

type RuleDomainsView = {
  ruleSetId: string
  ruleSetName: string
  query: string
  total: number
  matched: number
  limit: number
  truncated: boolean
  domains: RuleDomain[]
}

type Preview = {
  outputId: string
  slug: string
  nodeCount: number
  duplicateCount: number
  filteredCount: number
  unavailableCount: number
  regionCounts: Record<string, number>
  groups: Array<{ name: string; nodes: string[] }>
  sourceGroups: Array<{ name: string; nodes: string[] }>
  nodes: Array<{
    key: string
    name: string
    originalName: string
    sourceId: string
    source: string
    server: string
    port: number
    region: string
    regionCode: string
    resolvedIp: string
    exitIp: string
    delayMs: number
    alive?: boolean
    excludedReason: string
    regionSource: string
    probeStatus: string
    probeError: string
  }>
  excludedNodes: Preview["nodes"]
  generatedAt: string
  usedCachedSources: number
}

type Health = {
  ok: boolean
  needsAdminSetup: boolean
}

type Settings = {
  publicBaseUrl: string
  hasUserToken: boolean
  refreshMinutes: number
  trafficQueryMinutes: number
}

type User = {
  id: string
  slug: string
  name: string
  role: "admin" | "user"
  enabled: boolean
  createdAt?: string
  lastLoginAt?: string
}

type InviteCode = {
  id: string
  label: string
  createdAt: string
  usedAt?: string
  usedByUserId?: string
}

type CreatedInviteCode = InviteCode & {
  code: string
}

const defaultSource: Source = {
  id: "",
  name: "",
  url: "",
  sourceType: "url",
  fileName: "",
  fileContent: "",
  trafficQuery: {
    mode: "disabled",
    method: "GET",
    parser: { type: "json-path" },
  },
  trafficInfo: {
    uploadBytes: 0,
    downloadBytes: 0,
    totalBytes: 0,
    remainingBytes: 0,
    lastStatus: "",
    lastError: "",
  },
  urlMasked: "",
  enabled: true,
  remark: "",
  tags: [],
  lastStatus: "pending",
  lastFormat: "",
  lastNodeCount: 0,
  lastError: "",
  refreshProgress: "",
  refreshPercent: 0,
  nodeStats: { total: 0, alive: 0, dead: 0, unchecked: 0 },
  nodes: [],
}

const trafficTemplates = [
  {
    id: "subscription-userinfo",
    label: "订阅自带流量",
    description: "从订阅链接响应头 Subscription-Userinfo 读取 upload、download、total、expire。",
    query: {
      mode: "subscription-header" as const,
      method: "GET",
      parser: {
        type: "subscription-header" as const,
        total: "",
        download: "",
        remaining: "",
        upload: "",
        expire: "",
      },
    },
  },
  {
    id: "just-my-socks",
    label: "Just My Socks",
    description: "解析 monthly_bw_limit_b 与 bw_counter_b，自动计算剩余流量。",
    query: {
      mode: "custom-http" as const,
      method: "GET",
      parser: {
        type: "json-path" as const,
        total: "$.monthly_bw_limit_b",
        download: "$.bw_counter_b",
        remaining: "",
        upload: "",
        expire: "",
      },
    },
  },
]

const defaultOutput: Output = {
  id: "",
  slug: "main",
  name: "主订阅",
  enabled: true,
  format: "clash",
  sourceIds: [],
  filter: {
    includeKeywords: [],
    excludeKeywords: ["官网", "剩余", "过期", "流量"],
    regex: "",
  },
  renameRules: [],
  nodeNameOverrides: {},
  pac: {
    enabled: true,
    enabledSet: true,
    proxy: "PROXY 127.0.0.1:7890; SOCKS5 127.0.0.1:7890; DIRECT",
    ruleSetId: "china-direct",
    ruleSourceUrl: "https://cdn.jsdelivr.net/gh/ACL4SSR/ACL4SSR@master/Clash/ChinaDomain.list",
    ruleSourceFormat: "clash-domain",
    ruleRefreshHours: 24,
    domainKeywords: [],
    directDomainSuffixes: [],
    directCidrs: [],
    cachedDomainSuffixes: [],
  },
  groupMode: "region",
  lastNodeCount: 0,
  lastDroppedCount: 0,
  nodeNames: [],
}

const downloadFormatItems = [
  { label: "Mihomo YAML", value: "mihomo", filename: "mihomo.yaml" },
  { label: "Clash YAML", value: "clash", filename: "clash.yaml" },
  { label: "Base64 分享链接", value: "base64", filename: "subscription.txt" },
]

const outputFormatItems = [
  { label: "Clash / Mihomo YAML", value: "clash" },
  { label: "Base64 分享链接", value: "base64" },
]

const defaultRuleSource: RuleSource = {
  id: "acl4ssr-china-domain",
  name: "ACL4SSR 国内域名",
  url: "https://cdn.jsdelivr.net/gh/ACL4SSR/ACL4SSR@master/Clash/ChinaDomain.list",
  format: "clash-domain",
  refreshHours: 24,
  localPath: "rules/pac/acl4ssr-china-domain.list",
  cachedDomainSuffixes: [],
}

const defaultRuleSet: RuleSet = {
  id: "china-direct",
  name: "国内直连",
  sourceIds: ["acl4ssr-china-domain", "blackmatrix7-china", "loyalsoldier-direct"],
  domainKeywords: [],
  directDomainSuffixes: [],
  excludedDomainSuffixes: [],
  directCidrs: [],
  cachedDomainSuffixes: [],
}

function App() {
  const [health, setHealth] = useState<Health | null>(null)
  const [token, setToken] = useState(() => localStorage.getItem("sub-nest-token") ?? localStorage.getItem("subagg-token") ?? "")
  const [user, setUser] = useState<User | null>(() => readStoredUser())
  const [users, setUsers] = useState<User[]>([])
  const [targetUserId, setTargetUserId] = useState(() => localStorage.getItem("sub-nest-target-user") ?? "")
  const [dashboard, setDashboard] = useState<Dashboard | null>(null)
  const [sources, setSources] = useState<Source[]>([])
  const [ruleSources, setRuleSources] = useState<RuleSource[]>([])
  const [ruleSets, setRuleSets] = useState<RuleSet[]>([])
  const [outputs, setOutputs] = useState<Output[]>([])
  const [preview, setPreview] = useState<Preview | null>(null)
  const [settings, setSettings] = useState<Settings | null>(null)
  const [userToken, setUserToken] = useState(() => localStorage.getItem("sub-nest-user-token") ?? "")
  const [activeTab, setActiveTab] = useState("overview")
  const [loading, setLoading] = useState(true)
  const [busy, setBusy] = useState("")

  const api = useMemo(() => createAPI(token), [token])
  const authenticated = Boolean(token)
  const targetUserKnown = users.some((item) => item.id === targetUserId)
  const activeUserId = user?.role === "admin" ? (targetUserKnown ? targetUserId : user.id) : user?.id
  const activeUser = users.find((item) => item.id === activeUserId) ?? user
  const scopeQuery = user?.role === "admin" && activeUserId && activeUserId !== user.id ? `?userId=${encodeURIComponent(activeUserId)}` : ""
  const anyRefreshing = sources.some((source) => source.lastStatus === "refreshing")

  const loadProtected = useCallback(async (options?: { silent?: boolean }) => {
    if (!token) {
      return
    }
    if (!options?.silent) {
      setLoading(true)
    }
    try {
      const [nextUserResponse, nextUsers, nextDashboard, nextSources, nextRuleSources, nextRuleSets, nextOutputs, nextSettings] = await Promise.all([
        api.get<{ user: User }>("/api/me"),
        user?.role === "admin" ? api.get<User[]>("/api/admin/users") : Promise.resolve([]),
        api.get<Dashboard>(withScope("/api/dashboard", scopeQuery)),
        api.get<Source[]>(withScope("/api/sources?includeUrl=1", scopeQuery)),
        api.get<RuleSource[]>(withScope("/api/rule-sources", scopeQuery)),
        api.get<RuleSet[]>(withScope("/api/rule-sets", scopeQuery)),
        api.get<Output[]>(withScope("/api/outputs", scopeQuery)),
        api.get<Settings>("/api/settings"),
      ])
      setUser(nextUserResponse.user)
      localStorage.setItem("sub-nest-user", JSON.stringify(nextUserResponse.user))
      if (nextUserResponse.user.role === "admin") {
        const adminUsers = nextUsers.length > 0 ? nextUsers : [nextUserResponse.user]
        setUsers(adminUsers)
        if (!targetUserId || !adminUsers.some((item) => item.id === targetUserId)) {
          setTargetUserId(nextUserResponse.user.id)
          localStorage.setItem("sub-nest-target-user", nextUserResponse.user.id)
        }
      } else {
        setUsers([])
        setTargetUserId("")
        localStorage.removeItem("sub-nest-target-user")
      }
      setDashboard(nextDashboard)
      setSources(nextSources)
      setRuleSources(nextRuleSources)
      setRuleSets(nextRuleSets)
      setOutputs(nextOutputs)
      setSettings(nextSettings)
      if (!nextSettings.hasUserToken && userToken) {
        localStorage.removeItem("sub-nest-user-token")
        setUserToken("")
      }
      if (nextOutputs[0]) {
        try {
          setPreview(await api.get<Preview>(withScope(`/api/outputs/${nextOutputs[0].id}/preview`, scopeQuery)))
        } catch {
          setPreview(null)
        }
      }
    } catch (error) {
      localStorage.removeItem("subagg-token")
      localStorage.removeItem("sub-nest-token")
      localStorage.removeItem("sub-nest-user")
      localStorage.removeItem("sub-nest-target-user")
      setToken("")
      setUser(null)
      toast.error(messageOf(error))
    } finally {
      if (!options?.silent) {
        setLoading(false)
      }
    }
  }, [api, scopeQuery, targetUserId, token, user?.id, user?.role, userToken])

  useEffect(() => {
    createAPI("").get<Health>("/api/health").then(setHealth).catch(() => {
      setHealth({ ok: false, needsAdminSetup: false })
    })
  }, [])

  useEffect(() => {
    void loadProtected()
  }, [loadProtected])

  async function handleAuth(rawToken: string, setup: boolean, publicBaseURL: string) {
    setBusy("auth")
    try {
      const response = await createAPI("").post<{ token: string; user: User }>(
        setup ? "/api/setup" : "/api/login",
        setup ? { token: rawToken, publicBaseUrl: publicBaseURL } : { token: rawToken },
      )
      localStorage.setItem("sub-nest-token", response.token)
      localStorage.setItem("sub-nest-user", JSON.stringify(response.user))
      localStorage.setItem("sub-nest-target-user", response.user.id)
      localStorage.removeItem("subagg-token")
      setToken(response.token)
      setUser(response.user)
      setTargetUserId(response.user.id)
      setHealth({ ok: true, needsAdminSetup: false })
      toast.success(setup ? "管理员 token 已设置" : "登录成功")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  async function handleRegister(payload: { inviteCode: string; userSlug: string; name: string; token: string }) {
    setBusy("auth")
    try {
      const response = await createAPI("").post<{ token: string; user: User }>("/api/register", payload)
      localStorage.setItem("sub-nest-token", response.token)
      localStorage.setItem("sub-nest-user", JSON.stringify(response.user))
      localStorage.removeItem("subagg-token")
      localStorage.removeItem("sub-nest-target-user")
      setToken(response.token)
      setUser(response.user)
      setTargetUserId("")
      setHealth({ ok: true, needsAdminSetup: false })
      toast.success("注册成功")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  async function refreshAll() {
    setBusy("refresh-all")
    try {
      await api.post(withScope("/api/refresh", scopeQuery), {})
      toast.success("刷新任务已开始")
      await loadProtected()
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  if (!health) {
    return <LoadingShell />
  }

  if (!authenticated) {
    return (
      <TooltipProvider>
        <AuthScreen
          setup={health.needsAdminSetup}
          busy={busy === "auth"}
          onSubmit={handleAuth}
          onRegister={handleRegister}
        />
        <Toaster />
      </TooltipProvider>
    )
  }

  return (
    <TooltipProvider>
      <main className="min-h-screen bg-background text-foreground">
        <div className="mx-auto flex min-h-screen w-full max-w-[1440px] flex-col gap-4 px-4 py-4 sm:px-6 lg:px-8">
          <header className="flex flex-col gap-3 border-b pb-4 lg:flex-row lg:items-center lg:justify-between">
            <div className="flex min-w-0 items-center gap-3">
              <div className="flex size-9 items-center justify-center rounded-lg border bg-muted">
                <ShieldIcon />
              </div>
              <div className="min-w-0">
                <h1 className="truncate text-xl font-semibold">Sub Nest</h1>
                <p className="truncate text-sm text-muted-foreground">
                  固定公开地址，后台维护多个上游订阅源
                </p>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Badge variant="secondary">
                {dashboard?.enabledSources ?? 0} 个启用源
              </Badge>
              <Badge variant="outline">
                {dashboard?.totalCachedNodes ?? 0} 个缓存节点
              </Badge>
              {user ? (
                <Badge variant={user.role === "admin" ? "default" : "secondary"}>
                  <UserIcon />
                  {activeUser?.name || user.name}
                  {user.role === "admin" && activeUser?.id !== user.id ? " 的空间" : ""}
                </Badge>
              ) : null}
              <Button variant="outline" onClick={refreshAll} disabled={busy === "refresh-all" || anyRefreshing}>
                {busy === "refresh-all" || anyRefreshing ? <Loader2Icon data-icon="inline-start" /> : <RefreshCcwIcon data-icon="inline-start" />}
                {anyRefreshing ? "刷新中" : "刷新全部"}
              </Button>
              <Button
                variant="ghost"
                onClick={() => {
                  localStorage.removeItem("subagg-token")
                  localStorage.removeItem("sub-nest-token")
                  localStorage.removeItem("sub-nest-user")
                  localStorage.removeItem("sub-nest-target-user")
                  setToken("")
                  setUser(null)
                  setUsers([])
                  setTargetUserId("")
                  toast.success("已退出")
                }}
              >
                退出
              </Button>
            </div>
          </header>

          <Tabs value={activeTab} onValueChange={setActiveTab} className="min-h-0">
            <div className="flex flex-col gap-3 lg:flex-row">
              <TabsList className="w-full justify-start overflow-x-auto lg:w-fit">
                <TabsTrigger value="overview">首页</TabsTrigger>
                <TabsTrigger value="sources">订阅源</TabsTrigger>
                <TabsTrigger value="rules">规则</TabsTrigger>
                <TabsTrigger value="outputs">公开订阅</TabsTrigger>
                <TabsTrigger value="preview">预览</TabsTrigger>
                {user?.role === "admin" ? <TabsTrigger value="users">用户</TabsTrigger> : null}
                <TabsTrigger value="backup">备份</TabsTrigger>
                {user?.role === "admin" ? <TabsTrigger value="settings">设置</TabsTrigger> : null}
              </TabsList>
              {user?.role === "admin" ? (
                <Select
                  items={users.map((item) => ({ label: `${item.name || item.slug}${item.enabled ? "" : "（已禁用）"}`, value: item.id }))}
                  value={activeUserId}
                  onValueChange={(value) => {
                    const next = String(value)
                    setTargetUserId(next)
                    localStorage.setItem("sub-nest-target-user", next)
                    setPreview(null)
                  }}
                >
                  <SelectTrigger className="w-full lg:w-56">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {users.map((item) => (
                        <SelectItem key={item.id} value={item.id}>
                          {item.name || item.slug}{item.enabled ? "" : "（已禁用）"}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              ) : null}
            </div>

            <TabsContent value="overview">
              <Overview
                dashboard={dashboard}
                loading={loading}
                outputs={outputs}
                publicExampleUrl={dashboard?.publicExampleUrl}
                userToken={userToken}
              />
            </TabsContent>
            <TabsContent value="sources">
              <SourcesView
                api={api}
                sources={sources}
                loading={loading}
                busy={busy}
                setBusy={setBusy}
                scopeQuery={scopeQuery}
                reload={loadProtected}
              />
            </TabsContent>
            <TabsContent value="outputs">
              <OutputsView
                api={api}
                outputs={outputs}
                sources={sources}
                ruleSets={ruleSets}
                loading={loading}
                scopeQuery={scopeQuery}
                reload={loadProtected}
                publicExampleUrl={dashboard?.publicExampleUrl}
                userToken={userToken}
              />
            </TabsContent>
            <TabsContent value="rules">
              <RulesView
                api={api}
                ruleSources={ruleSources}
                ruleSets={ruleSets}
                outputs={outputs}
                scopeQuery={scopeQuery}
                reload={loadProtected}
              />
            </TabsContent>
            <TabsContent value="preview">
              <PreviewView
                api={api}
                outputs={outputs}
                preview={preview}
                setPreview={setPreview}
                scopeQuery={scopeQuery}
              />
            </TabsContent>
            {user?.role === "admin" ? (
              <TabsContent value="users">
                <UsersView api={api} users={users} reload={loadProtected} />
              </TabsContent>
            ) : null}
            <TabsContent value="backup">
              <BackupView api={api} reload={loadProtected} />
            </TabsContent>
            {user?.role === "admin" ? (
              <TabsContent value="settings">
                <SettingsView
                  api={api}
                  settings={settings}
                  userToken={userToken}
                  onAdminTokenUpdated={(nextToken) => {
                    localStorage.setItem("sub-nest-token", nextToken)
                    localStorage.removeItem("subagg-token")
                    setToken(nextToken)
                  }}
                  onUserTokenUpdated={(nextSettings, nextUserToken) => {
                    setSettings(nextSettings)
                    if (nextUserToken) {
                      localStorage.setItem("sub-nest-user-token", nextUserToken)
                      setUserToken(nextUserToken)
                    } else {
                      localStorage.removeItem("sub-nest-user-token")
                      setUserToken("")
                    }
                  }}
                />
              </TabsContent>
            ) : null}
          </Tabs>
        </div>
      </main>
      <Toaster />
    </TooltipProvider>
  )
}

function AuthScreen({
  setup,
  busy,
  onSubmit,
  onRegister,
}: {
  setup: boolean
  busy: boolean
  onSubmit: (token: string, setup: boolean, publicBaseURL: string) => void
  onRegister: (payload: { inviteCode: string; userSlug: string; name: string; token: string }) => void
}) {
  const [rawToken, setRawToken] = useState("")
  const [mode, setMode] = useState<"login" | "register">("login")
  const [inviteCode, setInviteCode] = useState("")
  const [userSlug, setUserSlug] = useState("")
  const [name, setName] = useState("")
  const [publicBaseURL, setPublicBaseURL] = useState(window.location.origin)
  const registering = !setup && mode === "register"
  const canSubmitAuth = !busy && rawToken.length >= 8 && (!registering || (inviteCode.trim() && userSlug.trim()))

  function submitAuth(event: FormEvent<HTMLFormElement>) {
    event.preventDefault()
    if (!canSubmitAuth) {
      return
    }
    if (registering) {
      onRegister({ inviteCode, userSlug, name, token: rawToken })
      return
    }
    onSubmit(rawToken, setup, publicBaseURL)
  }

  return (
    <main className="flex min-h-screen items-center justify-center bg-background p-4">
      <Card className="w-full max-w-md">
        <form onSubmit={submitAuth}>
        <CardHeader>
          <div className="flex size-10 items-center justify-center rounded-lg border bg-muted">
            {registering ? <KeyRoundIcon /> : <LockIcon />}
          </div>
          <CardTitle>{setup ? "初始化后台访问" : registering ? "使用授权码注册" : "登录后台"}</CardTitle>
          <CardDescription>
            {registering ? "注册后可使用自己的 token 独立维护订阅源和公开订阅。" : "后台使用本地 token 保护；订阅链接会在列表和日志中隐藏敏感部分。"}
          </CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            {!setup ? (
              <div className="grid grid-cols-2 gap-2">
                <Button type="button" variant={!registering ? "default" : "outline"} onClick={() => setMode("login")}>登录</Button>
                <Button type="button" variant={registering ? "default" : "outline"} onClick={() => setMode("register")}>注册</Button>
              </div>
            ) : null}
            {registering ? (
              <>
                <Field>
                  <FieldLabel htmlFor="invite-code">授权码</FieldLabel>
                  <Input id="invite-code" value={inviteCode} onChange={(event) => setInviteCode(event.target.value)} />
                </Field>
                <Field>
                  <FieldLabel htmlFor="user-slug">用户标识</FieldLabel>
                  <Input id="user-slug" value={userSlug} onChange={(event) => setUserSlug(event.target.value)} placeholder="例如 alice" />
                  <FieldDescription>仅使用小写字母、数字和短横线，用于生成 `/u/{userSlug || "alice"}/s/main`。</FieldDescription>
                </Field>
                <Field>
                  <FieldLabel htmlFor="display-name">显示名称</FieldLabel>
                  <Input id="display-name" value={name} onChange={(event) => setName(event.target.value)} placeholder="可选" />
                </Field>
              </>
            ) : null}
            <Field>
              <FieldLabel htmlFor="admin-token">{registering ? "个人 token" : "管理 token"}</FieldLabel>
              <PasswordInput
                id="admin-token"
                value={rawToken}
                onChange={(event) => setRawToken(event.target.value)}
                placeholder="至少 8 位"
              />
            </Field>
            {setup ? (
              <Field>
                <FieldLabel htmlFor="public-base">公开访问域名</FieldLabel>
                <Input
                  id="public-base"
                  value={publicBaseURL}
                  onChange={(event) => setPublicBaseURL(event.target.value)}
                />
                <FieldDescription>用于生成复制链接，可稍后在配置文件中调整。</FieldDescription>
              </Field>
            ) : null}
          </FieldGroup>
        </CardContent>
        <CardFooter>
          <Button
            type="submit"
            className="w-full"
            disabled={!canSubmitAuth}
          >
            {busy ? <Loader2Icon data-icon="inline-start" /> : <ShieldIcon data-icon="inline-start" />}
            {setup ? "完成初始化" : registering ? "创建账号" : "进入后台"}
          </Button>
        </CardFooter>
        </form>
      </Card>
    </main>
  )
}

function PasswordInput(props: ComponentProps<typeof Input>) {
  const [visible, setVisible] = useState(false)
  return (
    <div className="relative">
      <Input
        {...props}
        type={visible ? "text" : "password"}
        className={cn("pr-10", props.className)}
      />
      <Button
        type="button"
        variant="ghost"
        size="icon"
        className="absolute right-1 top-1/2 size-8 -translate-y-1/2 text-muted-foreground hover:text-foreground"
        onClick={() => setVisible((value) => !value)}
        aria-label={visible ? "隐藏输入内容" : "显示输入内容"}
      >
        {visible ? <EyeOffIcon /> : <EyeIcon />}
      </Button>
    </div>
  )
}

function Overview({
  dashboard,
  loading,
  outputs,
  publicExampleUrl,
  userToken,
}: {
  dashboard: Dashboard | null
  loading: boolean
  outputs: Output[]
  publicExampleUrl?: string
  userToken: string
}) {
  const stats = [
    { label: "公开订阅", value: dashboard?.outputCount ?? 0, sub: `${dashboard?.enabledOutputs ?? 0} 个启用` },
    { label: "订阅源", value: dashboard?.sourceCount ?? 0, sub: `${dashboard?.enabledSources ?? 0} 个启用` },
    { label: "正常源", value: dashboard?.healthySources ?? 0, sub: `${dashboard?.unhealthySources ?? 0} 个异常` },
    { label: "缓存节点", value: dashboard?.totalCachedNodes ?? 0, sub: formatTime(dashboard?.lastRefreshAt) },
  ]
  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {stats.map((stat) => (
          <Card key={stat.label}>
            <CardHeader className="pb-2">
              <CardDescription>{stat.label}</CardDescription>
              <CardTitle className="text-2xl">{loading ? <Skeleton className="h-8 w-16" /> : stat.value}</CardTitle>
            </CardHeader>
            <CardContent>
              <p className="text-sm text-muted-foreground">{stat.sub}</p>
            </CardContent>
          </Card>
        ))}
      </div>
      <Card>
        <CardHeader>
          <CardTitle>稳定订阅地址</CardTitle>
          <CardDescription>客户端只需要订阅固定地址，后续上游变化在后台维护。</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-3">
          {outputs.length === 0 ? (
            <Alert>
              <FileJsonIcon />
              <AlertTitle>还没有公开订阅</AlertTitle>
              <AlertDescription>进入“公开订阅”创建 `main` 后即可复制地址到 Clash / Mihomo。</AlertDescription>
            </Alert>
          ) : (
            outputs.map((output) => {
              const url = subscriptionURL(publicExampleUrl, output.slug, userToken)
              const pacUrl = pacURL(url)
              return (
                <div key={output.id} className="flex flex-col gap-2 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between">
                  <div className="min-w-0">
                    <div className="flex items-center gap-2">
                      <p className="truncate font-medium">{output.name}</p>
                      <StatusBadge status={output.enabled ? "ok" : "paused"} />
                    </div>
                    <p className="truncate text-sm text-muted-foreground">
                      {url}
                    </p>
                    <p className="truncate text-xs text-muted-foreground">
                      PAC {pacUrl}
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <Button variant="outline" size="sm" onClick={() => copyText(url, "订阅地址已复制")}>
                      <ClipboardIcon data-icon="inline-start" />
                      订阅
                    </Button>
                    <Button variant="outline" size="sm" onClick={() => copyText(pacUrl, "PAC 地址已复制")}>
                      <FileTextIcon data-icon="inline-start" />
                      PAC
                    </Button>
                  </div>
                </div>
              )
            })
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function SourcesView({
  api,
  sources,
  loading,
  busy,
  setBusy,
  scopeQuery,
  reload,
}: {
  api: API
  sources: Source[]
  loading: boolean
  busy: string
  setBusy: (value: string) => void
  scopeQuery: string
  reload: (options?: { silent?: boolean }) => Promise<void>
}) {
  const [editing, setEditing] = useState<Source | null>(null)
  const [expanded, setExpanded] = useState<Record<string, boolean>>({})

  function openSourceSheet(source: Source) {
    setEditing(source)
  }

  function toggleSourceNodes(source: Source) {
    const nextExpanded = !expanded[source.id]
    setExpanded({ ...expanded, [source.id]: nextExpanded })
  }

  async function refreshSource(source: Source) {
    setBusy(`refresh:${source.id}`)
    try {
      await api.post(withScope(`/api/sources/${source.id}/refresh`, scopeQuery), {})
      toast.success(`${source.name} 已开始刷新`)
      await reload({ silent: true })
    } catch (error) {
      toast.error(messageOf(error))
      await reload({ silent: true })
    } finally {
      setBusy("")
    }
  }

  useEffect(() => {
    if (!sources.some((source) => source.lastStatus === "refreshing")) {
      return
    }
    const timer = window.setInterval(() => {
      void reload({ silent: true })
    }, 2000)
    return () => window.clearInterval(timer)
  }, [reload, sources])

  async function deleteSource(source: Source) {
    if (!confirm(`删除订阅源「${source.name}」？`)) {
      return
    }
    try {
      await api.delete(withScope(`/api/sources/${source.id}`, scopeQuery))
      toast.success("订阅源已删除")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  return (
    <Card>
      <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <CardTitle>订阅源</CardTitle>
          <CardDescription>添加多个上游订阅，失败时会继续使用上一次成功缓存。</CardDescription>
        </div>
        <Button onClick={() => openSourceSheet(defaultSource)}>
          <PlusIcon data-icon="inline-start" />
          添加订阅源
        </Button>
      </CardHeader>
      <CardContent>
        {loading ? (
          <TableSkeleton />
        ) : sources.length === 0 ? (
          <Empty className="min-h-64">
            <EmptyHeader>
              <EmptyMedia variant="icon"><ServerIcon /></EmptyMedia>
              <EmptyTitle>还没有订阅源</EmptyTitle>
              <EmptyDescription>添加一个 base64、Clash 或明文分享链接订阅源开始聚合。</EmptyDescription>
            </EmptyHeader>
            <EmptyContent>
              <Button onClick={() => openSourceSheet(defaultSource)}>
                <PlusIcon data-icon="inline-start" />
                添加订阅源
              </Button>
            </EmptyContent>
          </Empty>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>名称</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>格式</TableHead>
                  <TableHead>节点</TableHead>
                  <TableHead>流量</TableHead>
                  <TableHead>订阅链接</TableHead>
                  <TableHead className="sticky right-0 z-10 min-w-52 bg-background text-right shadow-[-12px_0_16px_-16px_rgba(0,0,0,0.35)]">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {sources.map((source) => (
                  <Fragment key={source.id}>
                  <TableRow key={source.id}>
                    <TableCell className="min-w-44">
                      <div className="flex flex-col gap-1">
                        <div className="flex items-center gap-2">
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={expanded[source.id] ? "收起节点" : "展开节点"}
                            aria-expanded={expanded[source.id] ? "true" : "false"}
                            onClick={() => toggleSourceNodes(source)}
                          >
                            {expanded[source.id] ? <FiChevronDown /> : <FiChevronRight />}
                            <span className="sr-only">{expanded[source.id] ? "收起节点" : "展开节点"}</span>
                          </Button>
                          <span className="font-medium">{source.name}</span>
                        </div>
                        <span className="text-xs text-muted-foreground">{source.remark || "无备注"}</span>
                      </div>
                    </TableCell>
                    <TableCell className="min-w-56">
                      <div className="flex min-w-0 flex-col gap-1">
                        <div className="flex min-w-0 items-center gap-2">
                          <StatusBadge status={source.enabled ? source.lastStatus : "paused"} />
                          {source.refreshProgress ? (
                            <span className="max-w-56 truncate text-xs text-muted-foreground">{source.refreshProgress}</span>
                          ) : source.lastError ? (
                            <span className="max-w-40 truncate text-xs text-muted-foreground">{source.lastError}</span>
                          ) : null}
                        </div>
                        {source.lastStatus === "refreshing" ? (
                          <ProgressBar value={source.refreshPercent || 1} />
                        ) : null}
                      </div>
                    </TableCell>
                    <TableCell>{source.lastFormat || "-"}</TableCell>
                    <TableCell className="min-w-56">
                      <div className="flex w-max items-center gap-1.5 whitespace-nowrap">
                        <Badge variant="outline" className="font-mono tabular-nums">{source.lastNodeCount}</Badge>
                        <Badge variant="secondary">可用 {source.nodeStats?.alive ?? 0}</Badge>
                        <Badge variant={(source.nodeStats?.dead ?? 0) > 0 ? "destructive" : "outline"}>失败 {source.nodeStats?.dead ?? 0}</Badge>
                        <Badge variant="outline">未测 {source.nodeStats?.unchecked ?? 0}</Badge>
                      </div>
                    </TableCell>
                    <TableCell className="min-w-48">
                      <TrafficSummary source={source} />
                    </TableCell>
                    <TableCell className="max-w-64 truncate text-muted-foreground">{source.urlMasked}</TableCell>
                    <TableCell className="sticky right-0 z-10 min-w-52 bg-background shadow-[-12px_0_16px_-16px_rgba(0,0,0,0.35)]">
                      <div className="flex justify-end gap-2">
                        <Button variant="outline" size="sm" onClick={() => refreshSource(source)} disabled={busy === `refresh:${source.id}` || source.lastStatus === "refreshing"}>
                          {busy === `refresh:${source.id}` || source.lastStatus === "refreshing" ? <Loader2Icon data-icon="inline-start" /> : <RefreshCcwIcon data-icon="inline-start" />}
                          {source.lastStatus === "refreshing" ? "刷新中" : "刷新"}
                        </Button>
                        <Button variant="outline" size="sm" onClick={() => openSourceSheet(source)}>编辑</Button>
                        <Button variant="ghost" size="icon-sm" onClick={() => deleteSource(source)}>
                          <Trash2Icon />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                  {expanded[source.id] ? (
                    <TableRow key={`${source.id}-nodes`}>
                      <TableCell colSpan={7}>
                        <SourceNodesTable nodes={source.nodes ?? []} />
                      </TableCell>
                    </TableRow>
                  ) : null}
                  </Fragment>
                ))}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
      <SourceSheet
        api={api}
        scopeQuery={scopeQuery}
        source={editing}
        reload={reload}
        onOpenChange={(open) => !open && setEditing(null)}
        onSave={async (source) => {
          try {
            let saved: Source
            if (source.id) {
              saved = await api.put<Source>(withScope(`/api/sources/${source.id}`, scopeQuery), source)
            } else {
              saved = await api.post<Source>(withScope("/api/sources", scopeQuery), source)
            }
            toast.success("订阅源已保存")
            if ((saved.sourceType ?? "url") === "file") {
              await api.post(withScope(`/api/sources/${saved.id}/refresh`, scopeQuery), {})
              toast.success("文件订阅已开始解析")
            }
            setEditing(null)
            await reload()
          } catch (error) {
            toast.error(messageOf(error))
          }
        }}
      />
    </Card>
  )
}

function SourceNodesTable({ nodes }: { nodes: NonNullable<Source["nodes"]> }) {
  if (nodes.length === 0) {
    return (
      <Empty className="min-h-32">
        <EmptyHeader>
          <EmptyTitle>暂无节点缓存</EmptyTitle>
          <EmptyDescription>刷新订阅源后会显示原始节点和检测状态。</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }
  return (
    <div className="rounded-lg border bg-muted/20 p-2">
      <div className="overflow-x-auto">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>原始节点</TableHead>
              <TableHead>地址</TableHead>
              <TableHead>地区</TableHead>
              <TableHead>延迟</TableHead>
              <TableHead>状态</TableHead>
              <TableHead>出口 / 解析 IP</TableHead>
              <TableHead>失败原因</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {nodes.map((node) => (
              <TableRow key={`${node.key}-${node.server}-${node.port}`}>
                <TableCell className="max-w-64 truncate">{node.originalName || node.name}</TableCell>
                <TableCell className="max-w-64 truncate text-muted-foreground">{node.server}:{node.port}</TableCell>
                <TableCell>
                  <Badge variant={node.region === "其他节点" ? "outline" : "secondary"}>
                    {node.regionCode || "OTHER"}
                  </Badge>
                </TableCell>
                <TableCell className="font-mono text-muted-foreground tabular-nums">{formatDelay(node.delayMs)}</TableCell>
                <TableCell>
                  <Badge variant={node.alive === false ? "destructive" : node.alive === true ? "secondary" : "outline"}>
                    {node.alive === false ? "不可用" : node.alive === true ? "可用" : "未检测"}
                  </Badge>
                </TableCell>
                <TableCell className="text-muted-foreground">{node.exitIp || node.resolvedIp || "-"}</TableCell>
                <TableCell className="max-w-72 truncate text-muted-foreground">{node.probeError || node.excludedReason || "-"}</TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

function TrafficSummary({ source }: { source: Source }) {
  const mode = source.trafficQuery?.mode ?? "disabled"
  const info = source.trafficInfo
  if (mode === "disabled") {
    return <span className="text-sm text-muted-foreground">未配置</span>
  }
  if (info?.lastStatus === "error") {
    return (
      <div className="flex max-w-52 flex-col gap-1">
        <Badge variant="destructive">查询失败</Badge>
        <span className="truncate text-xs text-muted-foreground">{info.lastError}</span>
      </div>
    )
  }
  const remaining = info?.remainingBytes ? formatBytes(info.remainingBytes) : "-"
  const total = info?.totalBytes ? formatBytes(info.totalBytes) : ""
  return (
    <div className="flex flex-col gap-1">
      <div className="flex flex-wrap gap-1.5">
        <Badge variant={info?.lastStatus === "ok" ? "secondary" : "outline"}>
          剩余 {remaining}{total ? ` / ${total}` : ""}
        </Badge>
      </div>
      <span className="text-xs text-muted-foreground">
        {info?.expireAt ? `到期 ${formatDate(info.expireAt)}` : info?.lastCheckedAt ? `查询 ${formatTime(info.lastCheckedAt)}` : "尚未查询"}
      </span>
    </div>
  )
}

function ProgressBar({ value }: { value: number }) {
  const percent = Math.max(1, Math.min(100, Math.round(value)))
  return (
    <div className="h-1.5 w-full overflow-hidden rounded-full bg-muted" aria-label={`刷新进度 ${percent}%`}>
      <div className="h-full rounded-full bg-primary transition-all" style={{ width: `${percent}%` }} />
    </div>
  )
}

function SourceSheet({
  api,
  scopeQuery,
  source,
  reload,
  onOpenChange,
  onSave,
}: {
  api: API
  scopeQuery: string
  source: Source | null
  reload: (options?: { silent?: boolean }) => Promise<void>
  onOpenChange: (open: boolean) => void
  onSave: (source: Source) => Promise<void>
}) {
  const [draft, setDraft] = useState<Source>(defaultSource)
  const [trafficBusy, setTrafficBusy] = useState(false)
  const [trafficTemplateID, setTrafficTemplateID] = useState("")
  const open = Boolean(source)
  const sourceType = draft.sourceType ?? "url"
  const trafficQuery = draft.trafficQuery ?? defaultSource.trafficQuery
  const trafficParser = trafficQuery.parser ?? {}

  useEffect(() => {
    setDraft(source ?? defaultSource)
    setTrafficTemplateID("")
  }, [source])

  async function loadSourceFile(file?: File) {
    if (!file) {
      return
    }
    const content = await file.text()
    setDraft((current) => ({
      ...current,
      sourceType: "file",
      url: "",
      fileName: file.name,
      fileContent: content,
      name: current.name || file.name.replace(/\.[^.]+$/, ""),
    }))
    toast.success(`已读取文件「${file.name}」`)
  }

  function updateTrafficQuery(next: Partial<TrafficQuery>) {
    setDraft((current) => ({
      ...current,
      trafficQuery: {
        ...(current.trafficQuery ?? defaultSource.trafficQuery),
        ...next,
      },
    }))
  }

  function updateTrafficParser(next: Partial<NonNullable<TrafficQuery["parser"]>>) {
    setDraft((current) => ({
      ...current,
      trafficQuery: {
        ...(current.trafficQuery ?? defaultSource.trafficQuery),
        parser: {
          ...(current.trafficQuery?.parser ?? {}),
          ...next,
        },
      },
    }))
  }

  function updateTrafficHeaders(value: string) {
    const headers: Record<string, string> = {}
    for (const line of value.split("\n")) {
      const [key, ...rest] = line.split(":")
      if (key?.trim()) {
        headers[key.trim()] = rest.join(":").trim()
      }
    }
    updateTrafficQuery({ headers })
  }

  function applyTrafficTemplate(templateID: string) {
    const template = trafficTemplates.find((item) => item.id === templateID)
    if (!template) {
      return
    }
    setTrafficTemplateID(templateID)
    setDraft((current) => {
      const currentQuery = current.trafficQuery ?? defaultSource.trafficQuery
      return {
        ...current,
        trafficQuery: {
          ...currentQuery,
          ...template.query,
          url: currentQuery.url,
          headers: currentQuery.headers,
          body: currentQuery.body,
          parser: {
            ...(template.query.parser ?? {}),
          },
        },
      }
    })
  }

  async function testTrafficQuery() {
    setTrafficBusy(true)
    try {
      const updated = draft.id
        ? await api.post<Source>(withScope(`/api/sources/${draft.id}/traffic-query`, scopeQuery), { source: draft })
        : { trafficInfo: await api.post<TrafficInfo>(withScope("/api/traffic-query/test", scopeQuery), { source: draft }) }
      setDraft((current) => ({
        ...current,
        trafficInfo: updated.trafficInfo,
      }))
      if (updated.trafficInfo?.lastStatus === "error") {
        toast.error(updated.trafficInfo.lastError || "流量查询失败")
      } else {
        toast.success("流量查询完成")
      }
      if (draft.id) {
        await reload({ silent: true })
      }
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setTrafficBusy(false)
    }
  }

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-lg">
        <SheetHeader>
          <SheetTitle>{draft.id ? "编辑订阅源" : "添加订阅源"}</SheetTitle>
          <SheetDescription>订阅链接或文件仅用于后台解析，列表中默认展示脱敏地址或文件名。</SheetDescription>
        </SheetHeader>
        <div className="px-4">
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="source-name">名称</FieldLabel>
              <Input id="source-name" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
            </Field>
            <Field>
              <FieldLabel>来源类型</FieldLabel>
              <div className="flex flex-wrap gap-2">
                <Button
                  type="button"
                  variant={sourceType === "url" ? "default" : "outline"}
                  size="sm"
                  onClick={() => setDraft({ ...draft, sourceType: "url", fileName: "", fileContent: "" })}
                >
                  订阅链接
                </Button>
                <Button
                  type="button"
                  variant={sourceType === "file" ? "default" : "outline"}
                  size="sm"
                  onClick={() => setDraft({ ...draft, sourceType: "file", url: "" })}
                >
                  上传文件
                </Button>
              </div>
            </Field>
            <Field>
              <FieldLabel htmlFor={sourceType === "file" ? "source-file" : "source-url"}>
                {sourceType === "file" ? "订阅文件" : "订阅链接"}
              </FieldLabel>
              {sourceType === "file" ? (
                <div className="flex flex-col gap-2">
                  <Input
                    id="source-file"
                    type="file"
                    accept=".yaml,.yml,.txt,.conf,.list,.json,text/plain,application/x-yaml,application/yaml"
                    onChange={(event) => void loadSourceFile(event.target.files?.[0])}
                  />
                  <FieldDescription>
                    {draft.fileName ? `已选择：${draft.fileName}` : "支持 Clash/Mihomo YAML、base64 或明文分享链接文件。"}
                  </FieldDescription>
                </div>
              ) : (
                <Textarea id="source-url" className="min-h-24" value={draft.url ?? ""} onChange={(event) => setDraft({ ...draft, sourceType: "url", url: event.target.value })} />
              )}
            </Field>
            <Field>
              <FieldLabel htmlFor="source-tags">标签</FieldLabel>
              <Input id="source-tags" value={draft.tags.join(", ")} onChange={(event) => setDraft({ ...draft, tags: splitList(event.target.value) })} placeholder="机场A, 高速" />
            </Field>
            <Field>
              <FieldLabel htmlFor="source-remark">备注</FieldLabel>
              <Textarea id="source-remark" value={draft.remark} onChange={(event) => setDraft({ ...draft, remark: event.target.value })} />
            </Field>
            <Field orientation="horizontal">
              <div>
                <FieldLabel>启用订阅源</FieldLabel>
                <FieldDescription>停用后不会参与任何公开订阅输出。</FieldDescription>
              </div>
              <Switch checked={draft.enabled} onCheckedChange={(checked) => setDraft({ ...draft, enabled: checked })} />
            </Field>
            <Separator />
            <Field>
              <FieldLabel>流量查询</FieldLabel>
              <Select
                value={trafficQuery.mode || "disabled"}
                onValueChange={(value) => updateTrafficQuery({ mode: String(value) as TrafficQuery["mode"] })}
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectItem value="disabled">不查询</SelectItem>
                    <SelectItem value="subscription-header">读取订阅响应头</SelectItem>
                    <SelectItem value="subscription-body-regex">订阅正文正则</SelectItem>
                    <SelectItem value="custom-http">自定义 HTTP</SelectItem>
                  </SelectGroup>
                </SelectContent>
              </Select>
              <FieldDescription>刷新订阅源时会同步更新流量；查询失败不会影响节点缓存。</FieldDescription>
            </Field>
            <Field>
              <FieldLabel>流量模板</FieldLabel>
              <Select value={trafficTemplateID} onValueChange={(value) => applyTrafficTemplate(String(value))}>
                <SelectTrigger>
                  <SelectValue placeholder="选择模板" />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {trafficTemplates.map((template) => (
                      <SelectItem key={template.id} value={template.id}>{template.label}</SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
              <FieldDescription>
                选择模板会自动填充查询方式和解析字段，不会覆盖已输入的 URL、请求头或请求体。
              </FieldDescription>
            </Field>
            {trafficQuery.mode === "custom-http" ? (
              <>
                <Field>
                  <FieldLabel htmlFor="traffic-url">查询 URL</FieldLabel>
                  <Input id="traffic-url" value={trafficQuery.url ?? ""} onChange={(event) => updateTrafficQuery({ url: event.target.value })} />
                </Field>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Field>
                    <FieldLabel>请求方法</FieldLabel>
                    <Select value={trafficQuery.method || "GET"} onValueChange={(value) => updateTrafficQuery({ method: String(value) })}>
                      <SelectTrigger><SelectValue /></SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          <SelectItem value="GET">GET</SelectItem>
                          <SelectItem value="POST">POST</SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </Field>
                  <Field>
                    <FieldLabel>解析方式</FieldLabel>
                    <Select value={trafficParser.type || "json-path"} onValueChange={(value) => updateTrafficParser({ type: String(value) as NonNullable<TrafficQuery["parser"]>["type"] })}>
                      <SelectTrigger><SelectValue /></SelectTrigger>
                      <SelectContent>
                        <SelectGroup>
                          <SelectItem value="json-path">JSON 路径</SelectItem>
                          <SelectItem value="regex">正则</SelectItem>
                          <SelectItem value="subscription-header">响应头</SelectItem>
                        </SelectGroup>
                      </SelectContent>
                    </Select>
                  </Field>
                </div>
                <Field>
                  <FieldLabel htmlFor="traffic-headers">请求头</FieldLabel>
                  <Textarea
                    id="traffic-headers"
                    className="min-h-20 font-mono text-xs"
                    value={headersToText(trafficQuery.headers)}
                    onChange={(event) => updateTrafficHeaders(event.target.value)}
                    placeholder={"Authorization: Bearer xxx\nCookie: uid=..."}
                  />
                </Field>
                {trafficQuery.method === "POST" ? (
                  <Field>
                    <FieldLabel htmlFor="traffic-body">请求体</FieldLabel>
                    <Textarea id="traffic-body" className="min-h-20 font-mono text-xs" value={trafficQuery.body ?? ""} onChange={(event) => updateTrafficQuery({ body: event.target.value })} />
                  </Field>
                ) : null}
              </>
            ) : null}
            {trafficQuery.mode === "subscription-body-regex" || (trafficQuery.mode === "custom-http" && (trafficParser.type ?? "json-path") !== "subscription-header") ? (
              <div className="grid gap-3 sm:grid-cols-2">
                <Field>
                  <FieldLabel htmlFor="traffic-remaining">{trafficParser.type === "regex" || trafficQuery.mode === "subscription-body-regex" ? "剩余正则" : "剩余 JSON 路径"}</FieldLabel>
                  <Input id="traffic-remaining" value={trafficParser.remaining ?? ""} onChange={(event) => updateTrafficParser({ remaining: event.target.value })} placeholder={trafficParser.type === "regex" ? "剩余[:：]\\s*([0-9.]+\\s*GB)" : "$.data.remaining"} />
                </Field>
                <Field>
                  <FieldLabel htmlFor="traffic-total">{trafficParser.type === "regex" || trafficQuery.mode === "subscription-body-regex" ? "总量正则" : "总量 JSON 路径"}</FieldLabel>
                  <Input id="traffic-total" value={trafficParser.total ?? ""} onChange={(event) => updateTrafficParser({ total: event.target.value })} placeholder={trafficParser.type === "regex" ? "总量[:：]\\s*([0-9.]+\\s*GB)" : "$.data.total"} />
                </Field>
                <Field>
                  <FieldLabel htmlFor="traffic-upload">{trafficParser.type === "regex" || trafficQuery.mode === "subscription-body-regex" ? "上传正则" : "上传 JSON 路径"}</FieldLabel>
                  <Input id="traffic-upload" value={trafficParser.upload ?? ""} onChange={(event) => updateTrafficParser({ upload: event.target.value })} placeholder={trafficParser.type === "regex" ? "上传[:：]\\s*([0-9.]+\\s*GB)" : "$.data.upload"} />
                </Field>
                <Field>
                  <FieldLabel htmlFor="traffic-download">{trafficParser.type === "regex" || trafficQuery.mode === "subscription-body-regex" ? "下载正则" : "下载 JSON 路径"}</FieldLabel>
                  <Input id="traffic-download" value={trafficParser.download ?? ""} onChange={(event) => updateTrafficParser({ download: event.target.value })} placeholder={trafficParser.type === "regex" ? "下载[:：]\\s*([0-9.]+\\s*GB)" : "$.data.download"} />
                </Field>
                <Field className="sm:col-span-2">
                  <FieldLabel htmlFor="traffic-expire">{trafficParser.type === "regex" || trafficQuery.mode === "subscription-body-regex" ? "到期正则" : "到期 JSON 路径"}</FieldLabel>
                  <Input id="traffic-expire" value={trafficParser.expire ?? ""} onChange={(event) => updateTrafficParser({ expire: event.target.value })} placeholder={trafficParser.type === "regex" ? "到期[:：]\\s*([0-9-]+)" : "$.data.expire"} />
                </Field>
              </div>
            ) : null}
            {trafficQuery.mode !== "disabled" ? (
              <Alert>
                <DatabaseBackupIcon />
                <AlertTitle>{trafficStatusTitle(draft.trafficInfo)}</AlertTitle>
                <AlertDescription className="flex flex-col gap-1">
                  <span>{trafficInfoText(draft.trafficInfo)}</span>
                  {draft.trafficInfo?.lastError ? <span className="text-destructive">{draft.trafficInfo.lastError}</span> : null}
                  <TrafficDebugPanel info={draft.trafficInfo} />
                </AlertDescription>
              </Alert>
            ) : null}
          </FieldGroup>
        </div>
        <SheetFooter className="flex flex-wrap gap-2">
          {trafficQuery.mode !== "disabled" ? (
            <Button type="button" variant="outline" disabled={trafficBusy || !sourceInputReady(draft)} onClick={testTrafficQuery}>
              {trafficBusy ? <Loader2Icon data-icon="inline-start" /> : <RefreshCcwIcon data-icon="inline-start" />}
              测试流量查询
            </Button>
          ) : null}
          <Button disabled={!draft.name || (sourceType === "file" ? !draft.fileContent : !draft.url)} onClick={() => onSave(draft)}>
            <SaveIcon data-icon="inline-start" />
            保存
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function OutputsView({
  api,
  outputs,
  sources,
  ruleSets,
  loading,
  scopeQuery,
  reload,
  publicExampleUrl,
  userToken,
}: {
  api: API
  outputs: Output[]
  sources: Source[]
  ruleSets: RuleSet[]
  loading: boolean
  scopeQuery: string
  reload: () => Promise<void>
  publicExampleUrl?: string
  userToken: string
}) {
  const [editing, setEditing] = useState<Output | null>(null)
  const preparedDefault = { ...defaultOutput, sourceIds: sources.map((source) => source.id), pac: { ...defaultOutput.pac, ruleSetId: ruleSets[0]?.id ?? defaultOutput.pac.ruleSetId } }

  function openOutputSheet(output: Output) {
    setEditing(output)
  }

  async function deleteOutput(output: Output) {
    if (!confirm(`删除公开订阅「${output.name}」？`)) {
      return
    }
    try {
      await api.delete(withScope(`/api/outputs/${output.id}`, scopeQuery))
      toast.success("公开订阅已删除")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  return (
    <Card>
      <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <CardTitle>公开订阅</CardTitle>
          <CardDescription>创建一个或多个稳定输出地址，分别选择订阅源与整理规则。</CardDescription>
        </div>
        <Button onClick={() => openOutputSheet(preparedDefault)}>
          <PlusIcon data-icon="inline-start" />
          创建公开订阅
        </Button>
      </CardHeader>
      <CardContent>
        {loading ? (
          <TableSkeleton />
        ) : outputs.length === 0 ? (
          <Empty className="min-h-64">
            <EmptyHeader>
              <EmptyMedia variant="icon"><ActivityIcon /></EmptyMedia>
              <EmptyTitle>还没有公开订阅</EmptyTitle>
              <EmptyDescription>建议先创建 slug 为 `main` 的默认输出地址。</EmptyDescription>
            </EmptyHeader>
          </Empty>
        ) : (
          <div className="grid gap-3 lg:grid-cols-2">
            {outputs.map((output) => {
              const url = subscriptionURL(publicExampleUrl, output.slug, userToken)
              const pacUrl = pacURL(url)
              return (
                <Card key={output.id}>
                  <CardHeader>
                    <div className="flex items-start justify-between gap-3">
                      <div className="min-w-0">
                        <CardTitle className="truncate">{output.name}</CardTitle>
                        <CardDescription className="truncate">{url}</CardDescription>
                        <CardDescription className="truncate text-xs">PAC {pacUrl}</CardDescription>
                      </div>
                      <StatusBadge status={output.enabled ? "ok" : "paused"} />
                    </div>
                  </CardHeader>
                  <CardContent className="flex flex-col gap-3">
                    <div className="grid grid-cols-3 gap-2 text-sm">
                      <Metric label="格式" value={output.format} />
                      <Metric label="输入源" value={output.sourceIds.length} />
                      <Metric label="节点" value={output.lastNodeCount || "-"} />
                    </div>
                    <OutputNodeNames names={output.nodeNames ?? []} />
                    <div className="flex flex-wrap gap-2">
                      <Button variant="outline" size="sm" onClick={() => copyText(url, "公开订阅地址已复制")}>
                        <ClipboardIcon data-icon="inline-start" />
                        复制订阅链接
                      </Button>
                      <Button variant="outline" size="sm" onClick={() => copyText(pacUrl, "PAC 地址已复制")}>
                        <FileTextIcon data-icon="inline-start" />
                        复制 PAC
                      </Button>
                      <DropdownMenu>
                        <DropdownMenuTrigger render={<Button variant="outline" size="sm" />}>
                          <DownloadIcon data-icon="inline-start" />
                          下载配置
                        </DropdownMenuTrigger>
                        <DropdownMenuContent align="start" className="w-52">
                          <DropdownMenuGroup>
                            {downloadFormatItems.map((item) => (
                              <DropdownMenuItem
                                key={item.value}
                                onClick={() => downloadSubscription(url, output.slug, item.value)}
                              >
                                <DownloadIcon />
                                {item.label}
                              </DropdownMenuItem>
                            ))}
                            <DropdownMenuItem onClick={() => downloadPAC(pacUrl, output.slug)}>
                              <FileTextIcon />
                              PAC 文件
                            </DropdownMenuItem>
                          </DropdownMenuGroup>
                        </DropdownMenuContent>
                      </DropdownMenu>
                      <Button variant="outline" size="sm" onClick={() => openOutputSheet(output)}>编辑</Button>
                      <Button variant="ghost" size="icon-sm" onClick={() => deleteOutput(output)}>
                        <Trash2Icon />
                      </Button>
                    </div>
                  </CardContent>
                </Card>
              )
            })}
          </div>
        )}
      </CardContent>
      <OutputSheet
        output={editing}
        sources={sources}
        ruleSets={ruleSets}
        onOpenChange={(open) => !open && setEditing(null)}
        onSave={async (output) => {
          try {
            if (output.id) {
              await api.put(withScope(`/api/outputs/${output.id}`, scopeQuery), output)
            } else {
              await api.post(withScope("/api/outputs", scopeQuery), output)
            }
            toast.success("公开订阅已保存")
            setEditing(null)
            await reload()
          } catch (error) {
            toast.error(messageOf(error))
          }
        }}
      />
    </Card>
  )
}

function OutputSheet({
  output,
  sources,
  ruleSets,
  onOpenChange,
  onSave,
}: {
  output: Output | null
  sources: Source[]
  ruleSets: RuleSet[]
  onOpenChange: (open: boolean) => void
  onSave: (output: Output) => Promise<void>
}) {
  const [draft, setDraft] = useState<Output>(defaultOutput)
  const open = Boolean(output)

  useEffect(() => {
    setDraft(normalizeOutputDraft(output ?? defaultOutput))
  }, [output])

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent className="w-full overflow-y-auto sm:max-w-xl">
        <SheetHeader>
          <SheetTitle>{draft.id ? "编辑公开订阅" : "创建公开订阅"}</SheetTitle>
          <SheetDescription>过滤、重命名和自动分组会在输出时统一应用。</SheetDescription>
        </SheetHeader>
        <div className="px-4">
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="output-name">名称</FieldLabel>
              <Input id="output-name" value={draft.name} onChange={(event) => setDraft({ ...draft, name: event.target.value })} />
            </Field>
            <Field>
              <FieldLabel htmlFor="output-slug">公开 slug</FieldLabel>
              <Input id="output-slug" value={draft.slug} onChange={(event) => setDraft({ ...draft, slug: event.target.value })} />
              <FieldDescription>最终地址形如 `/s/main`。</FieldDescription>
            </Field>
            <Field>
              <FieldLabel>输出格式</FieldLabel>
              <Select
                items={outputFormatItems}
                value={draft.format}
                onValueChange={(value) => setDraft({ ...draft, format: String(value) })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    {outputFormatItems.map((item) => (
                      <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                    ))}
                  </SelectGroup>
                </SelectContent>
              </Select>
            </Field>
            {draft.format !== "base64" ? (
              <Field>
                <FieldLabel>节点分组</FieldLabel>
                <div className="rounded-lg border bg-muted/30 p-3 text-sm text-muted-foreground">
                  输出会同时生成地区组和原始订阅来源组；客户端可在“节点选择”中按地区或来源切换。
                </div>
              </Field>
            ) : null}
            <Field>
              <FieldLabel>输入订阅源</FieldLabel>
              <div className="flex flex-col gap-2 rounded-lg border p-3">
                {sources.length === 0 ? (
                  <p className="text-sm text-muted-foreground">还没有可选择的订阅源。</p>
                ) : (
                  sources.map((source) => (
                    <label key={source.id} className="flex items-center gap-2 text-sm">
                      <Checkbox
                        checked={draft.sourceIds.includes(source.id)}
                        onCheckedChange={(checked) => {
                          setDraft({
                            ...draft,
                            sourceIds: checked
                              ? [...draft.sourceIds, source.id]
                              : draft.sourceIds.filter((id) => id !== source.id),
                          })
                        }}
                      />
                      <span className="truncate">{source.name}</span>
                      <Badge variant="outline">{source.lastNodeCount}</Badge>
                    </label>
                  ))
                )}
              </div>
            </Field>
            <Field>
              <FieldLabel htmlFor="include-keywords">只保留关键词</FieldLabel>
              <Input id="include-keywords" value={draft.filter.includeKeywords.join(", ")} onChange={(event) => setDraft({ ...draft, filter: { ...draft.filter, includeKeywords: splitList(event.target.value) } })} />
            </Field>
            <Field>
              <FieldLabel htmlFor="exclude-keywords">排除关键词</FieldLabel>
              <Input id="exclude-keywords" value={draft.filter.excludeKeywords.join(", ")} onChange={(event) => setDraft({ ...draft, filter: { ...draft.filter, excludeKeywords: splitList(event.target.value) } })} />
            </Field>
            <Field>
              <FieldLabel htmlFor="regex">正则匹配</FieldLabel>
              <Input id="regex" value={draft.filter.regex} onChange={(event) => setDraft({ ...draft, filter: { ...draft.filter, regex: event.target.value } })} />
            </Field>
            <Field>
              <FieldLabel htmlFor="rename-rules">重命名规则</FieldLabel>
              <Textarea
                id="rename-rules"
                className="min-h-24 font-mono text-xs"
                value={draft.renameRules.map((rule) => `${rule.pattern} => ${rule.replacement}`).join("\n")}
                onChange={(event) => setDraft({ ...draft, renameRules: parseRenameRules(event.target.value) })}
                placeholder="香港 IEPL (\\d+) => [JMS] 香港 $1"
              />
            </Field>
            <Field>
              <FieldLabel>PAC 与直连方案</FieldLabel>
              <div className="grid gap-3 rounded-lg border p-3">
                <Field orientation="horizontal">
                  <div>
                    <FieldLabel>启用 PAC</FieldLabel>
                    <FieldDescription>关闭后 PAC 仍可访问，但只返回 DIRECT。</FieldDescription>
                  </div>
                  <Switch
                    checked={draft.pac.enabled}
                    onCheckedChange={(checked) => setDraft({ ...draft, pac: { ...draft.pac, enabled: checked, enabledSet: true } })}
                  />
                </Field>
                <Field>
                  <FieldLabel htmlFor="pac-proxy">代理返回值</FieldLabel>
                  <Input
                    id="pac-proxy"
                    value={draft.pac.proxy}
                    onChange={(event) => setDraft({ ...draft, pac: { ...draft.pac, proxy: event.target.value } })}
                    placeholder="PROXY 127.0.0.1:7890; SOCKS5 127.0.0.1:7890; DIRECT"
                  />
                  <FieldDescription>命中直连规则外的流量会返回这个值。</FieldDescription>
                </Field>
                <Field>
                  <FieldLabel>直连方案</FieldLabel>
                  <Select
                    value={draft.pac.ruleSetId || ruleSets[0]?.id || "china-direct"}
                    onValueChange={(value) => setDraft({ ...draft, pac: { ...draft.pac, ruleSetId: String(value) } })}
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="选择直连方案" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        {(ruleSets.length ? ruleSets : [defaultRuleSet]).map((set) => (
                          <SelectItem key={set.id} value={set.id}>{set.name}</SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                  <FieldDescription>每个公开订阅只选择一个方案；社区规则和手工直连名单在“规则”页维护。</FieldDescription>
                </Field>
                <div className="rounded-md bg-muted/35 p-2 text-xs text-muted-foreground">
                  方案缓存 {cachedDomainCount(ruleSets.find((set) => set.id === draft.pac.ruleSetId)) || cachedDomainCount(draft.pac)} 条
                  {ruleSets.find((set) => set.id === draft.pac.ruleSetId)?.lastSyncedAt ? `，最近同步 ${formatTime(ruleSets.find((set) => set.id === draft.pac.ruleSetId)?.lastSyncedAt)}` : ""}
                  {ruleSets.find((set) => set.id === draft.pac.ruleSetId)?.lastSyncStatus === "error" && ruleSets.find((set) => set.id === draft.pac.ruleSetId)?.lastSyncError ? `，失败：${ruleSets.find((set) => set.id === draft.pac.ruleSetId)?.lastSyncError}` : ""}
                </div>
              </div>
            </Field>
            <Field orientation="horizontal">
              <div>
                <FieldLabel>启用公开地址</FieldLabel>
                <FieldDescription>暂停后 `/s/{draft.slug || "slug"}` 会拒绝访问。</FieldDescription>
              </div>
              <Switch checked={draft.enabled} onCheckedChange={(checked) => setDraft({ ...draft, enabled: checked })} />
            </Field>
          </FieldGroup>
        </div>
        <SheetFooter>
          <Button disabled={!draft.name || !draft.slug} onClick={() => onSave(draft)}>
            <SaveIcon data-icon="inline-start" />
            保存
          </Button>
        </SheetFooter>
      </SheetContent>
    </Sheet>
  )
}

function PreviewView({
  api,
  outputs,
  preview,
  setPreview,
  scopeQuery,
}: {
  api: API
  outputs: Output[]
  preview: Preview | null
  setPreview: (preview: Preview | null) => void
  scopeQuery: string
}) {
  const [selected, setSelected] = useState(outputs[0]?.id ?? "")
  const [loading, setLoading] = useState(false)
  const [nameDrafts, setNameDrafts] = useState<Record<string, string>>({})

  useEffect(() => {
    if (!selected && outputs[0]?.id) {
      setSelected(outputs[0].id)
    }
  }, [outputs, selected])

  useEffect(() => {
    if (!preview) {
      return
    }
    setNameDrafts(Object.fromEntries(preview.nodes.map((node) => [node.key, node.name])))
  }, [preview])

  async function loadPreview(id = selected) {
    if (!id) {
      return
    }
    setLoading(true)
    try {
      setPreview(await api.get<Preview>(withScope(`/api/outputs/${id}/preview`, scopeQuery)))
      toast.success("预览已生成")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setLoading(false)
    }
  }

  async function saveNodeNames() {
    const output = outputs.find((item) => item.id === selected)
    if (!output) {
      return
    }
    const cleaned = Object.fromEntries(
      Object.entries(nameDrafts).map(([key, value]) => [key, value.trim()]).filter(([, value]) => value),
    )
    try {
      await api.put(withScope(`/api/outputs/${output.id}`, scopeQuery), {
        ...output,
        nodeNameOverrides: cleaned,
      })
      toast.success("节点名称已保存")
      await loadPreview(output.id)
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  if (outputs.length === 0) {
    return (
      <Empty className="min-h-72 border">
        <EmptyHeader>
          <EmptyMedia variant="icon"><EyeIcon /></EmptyMedia>
          <EmptyTitle>暂无可预览的公开订阅</EmptyTitle>
          <EmptyDescription>创建公开订阅后，这里会展示去重、过滤和地区分组结果。</EmptyDescription>
        </EmptyHeader>
      </Empty>
    )
  }

  const selectItems = outputs.map((output) => ({ label: output.name, value: output.id }))

  return (
    <div className="flex flex-col gap-4">
      <Card>
        <CardHeader className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <CardTitle>输出预览</CardTitle>
            <CardDescription>检查最终节点数量、被过滤节点和地区分组。</CardDescription>
          </div>
          <div className="flex gap-2">
            <Select items={selectItems} value={selected} onValueChange={(value) => setSelected(String(value))}>
              <SelectTrigger className="w-52">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectGroup>
                  {selectItems.map((item) => (
                    <SelectItem key={item.value} value={item.value}>{item.label}</SelectItem>
                  ))}
                </SelectGroup>
              </SelectContent>
            </Select>
            <Button variant="outline" onClick={() => loadPreview()} disabled={loading}>
              {loading ? <Loader2Icon data-icon="inline-start" /> : <RefreshCcwIcon data-icon="inline-start" />}
              生成预览
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {!preview ? (
            <Empty className="min-h-48">
              <EmptyHeader>
                <EmptyTitle>选择一个公开订阅生成预览</EmptyTitle>
              </EmptyHeader>
            </Empty>
          ) : (
            <div className="flex flex-col gap-4">
              <div className="grid gap-3 sm:grid-cols-4">
                <Metric label="输出节点" value={preview.nodeCount} />
                <Metric label="重复移除" value={preview.duplicateCount} />
                <Metric label="不可用移除" value={preview.unavailableCount} />
                <Metric label="使用缓存源" value={preview.usedCachedSources} />
              </div>
              <Separator />
              <div className="grid gap-3 lg:grid-cols-2">
                {preview.groups.map((group) => (
                  <Card key={`region-${group.name}`}>
                    <CardHeader>
                      <CardTitle className="text-base">{group.name}</CardTitle>
                      <CardDescription>{group.nodes.length} 个节点</CardDescription>
                    </CardHeader>
                  </Card>
                ))}
                {(preview.sourceGroups ?? []).map((group) => (
                  <Card key={`source-${group.name}`}>
                    <CardHeader>
                      <CardTitle className="text-base">{group.name}</CardTitle>
                      <CardDescription>{group.nodes.length} 个节点</CardDescription>
                    </CardHeader>
                  </Card>
                ))}
              </div>
              <Card>
                <CardHeader>
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
                    <div>
                      <CardTitle className="text-base">节点名称与地址识别</CardTitle>
                      <CardDescription>默认按地区编号命名；可为单个节点覆盖输出名称。</CardDescription>
                    </div>
                    <Button variant="outline" size="sm" onClick={saveNodeNames}>
                      <SaveIcon data-icon="inline-start" />
                      保存名称
                    </Button>
                  </div>
                </CardHeader>
                <CardContent>
                  <div className="overflow-x-auto">
                    <Table>
                      <TableHeader>
                        <TableRow>
                          <TableHead>输出名称</TableHead>
                          <TableHead>原始名称</TableHead>
                          <TableHead>地址</TableHead>
                          <TableHead>地区</TableHead>
                          <TableHead>延迟</TableHead>
                          <TableHead>可用性</TableHead>
                          <TableHead>出口 / 解析 IP</TableHead>
                          <TableHead>来源</TableHead>
                        </TableRow>
                      </TableHeader>
                      <TableBody>
                        {preview.nodes.slice(0, 80).map((node) => (
                          <TableRow key={`${node.name}-${node.server}-${node.port}`}>
                            <TableCell className="min-w-44">
                              <Input
                                value={nameDrafts[node.key] ?? node.name}
                                onChange={(event) => setNameDrafts({ ...nameDrafts, [node.key]: event.target.value })}
                              />
                            </TableCell>
                            <TableCell className="max-w-64 truncate text-muted-foreground">{node.originalName || node.name}</TableCell>
                            <TableCell className="max-w-72 truncate text-muted-foreground">{node.server}:{node.port}</TableCell>
                            <TableCell>
                              <Badge variant={node.region === "其他节点" ? "outline" : "secondary"}>
                                {node.regionCode || "OTHER"}
                              </Badge>
                            </TableCell>
                            <TableCell className="font-mono text-muted-foreground tabular-nums">{formatDelay(node.delayMs)}</TableCell>
                            <TableCell>
                              <Badge variant={node.alive === false ? "destructive" : node.alive === true ? "secondary" : "outline"}>
                                {node.alive === false ? "不可用" : node.alive === true ? "可用" : "未检测"}
                              </Badge>
                            </TableCell>
                            <TableCell className="text-muted-foreground">{node.exitIp || node.resolvedIp || "-"}</TableCell>
                            <TableCell className="max-w-48 truncate text-muted-foreground">
                              {node.source || node.sourceId || "-"}
                            </TableCell>
                          </TableRow>
                        ))}
                      </TableBody>
                    </Table>
                  </div>
                  {preview.nodes.length > 80 ? (
                    <p className="mt-3 text-sm text-muted-foreground">仅展示前 80 个节点。</p>
                  ) : null}
                </CardContent>
              </Card>
              {preview.excludedNodes.length > 0 ? (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-base">已排除不可用节点</CardTitle>
                    <CardDescription>这些节点不会进入公开订阅输出。</CardDescription>
                  </CardHeader>
                  <CardContent>
                    <div className="flex flex-col gap-2 text-sm">
                      {preview.excludedNodes.map((node) => (
                        <div key={node.key} className="flex flex-col gap-1 rounded-lg border p-3 sm:flex-row sm:items-center sm:justify-between">
                          <span className="truncate font-medium">{node.originalName || node.name}</span>
                          <span className="truncate text-muted-foreground">{node.excludedReason || node.probeError || "不可用"}</span>
                        </div>
                      ))}
                    </div>
                  </CardContent>
                </Card>
              ) : null}
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function BackupView({ api, reload }: { api: API; reload: () => Promise<void> }) {
  const [backupText, setBackupText] = useState("")

  async function downloadBackup() {
    try {
      const response = await fetch("/api/backup", {
        headers: { Authorization: `Bearer ${api.token}` },
      })
      if (!response.ok) {
        throw new Error("备份下载失败")
      }
      const blob = await response.blob()
      const url = URL.createObjectURL(blob)
      const link = document.createElement("a")
      link.href = url
      link.download = "sub-nest-backup.json"
      link.click()
      URL.revokeObjectURL(url)
      toast.success("备份已开始下载")
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  async function restore() {
    try {
      const parsed = JSON.parse(backupText)
      await api.post("/api/restore", parsed)
      toast.success("配置已恢复")
      setBackupText("")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>导出备份</CardTitle>
          <CardDescription>导出完整配置、订阅源状态和缓存节点，用于迁移或恢复。</CardDescription>
        </CardHeader>
        <CardContent>
          <Alert>
            <DatabaseBackupIcon />
            <AlertTitle>备份文件包含完整订阅链接</AlertTitle>
            <AlertDescription>请把文件保存在私密位置，不要发到公开仓库或聊天群。</AlertDescription>
          </Alert>
        </CardContent>
        <CardFooter>
          <Button onClick={downloadBackup}>
            <DatabaseBackupIcon data-icon="inline-start" />
            下载备份
          </Button>
        </CardFooter>
      </Card>
      <Card>
        <CardHeader>
          <CardTitle>导入恢复</CardTitle>
          <CardDescription>粘贴备份 JSON 后恢复当前配置。</CardDescription>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="backup-json">备份 JSON</FieldLabel>
              <Textarea id="backup-json" className="min-h-48 font-mono text-xs" value={backupText} onChange={(event) => setBackupText(event.target.value)} />
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter>
          <Button variant="outline" disabled={!backupText.trim()} onClick={restore}>
            <ArchiveRestoreIcon data-icon="inline-start" />
            恢复配置
          </Button>
        </CardFooter>
      </Card>
    </div>
  )
}

function UsersView({ api, users, reload }: { api: API; users: User[]; reload: () => Promise<void> }) {
  const [inviteCodes, setInviteCodes] = useState<InviteCode[]>([])
  const [label, setLabel] = useState("")
  const [createdCode, setCreatedCode] = useState("")
  const [loading, setLoading] = useState(false)

  const loadInvites = useCallback(async () => {
    try {
      setInviteCodes(await api.get<InviteCode[]>("/api/admin/invite-codes"))
    } catch (error) {
      toast.error(messageOf(error))
    }
  }, [api])

  useEffect(() => {
    void loadInvites()
  }, [loadInvites])

  async function createInvite() {
    setLoading(true)
    try {
      const invite = await api.post<CreatedInviteCode>("/api/admin/invite-codes", { label })
      setCreatedCode(invite.code)
      setLabel("")
      toast.success("授权码已创建")
      await loadInvites()
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setLoading(false)
    }
  }

  async function deleteInvite(invite: InviteCode) {
    if (!window.confirm(`确定删除授权码「${invite.label || invite.id}」吗？`)) {
      return
    }
    try {
      await api.delete(`/api/admin/invite-codes/${invite.id}`)
      toast.success("授权码已删除")
      await loadInvites()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  async function setUserEnabled(user: User, enabled: boolean) {
    try {
      await api.put(`/api/admin/users/${user.id}`, { enabled })
      toast.success(enabled ? "用户已启用" : "用户已禁用")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    }
  }

  const userName = (id?: string) => users.find((item) => item.id === id)?.name || users.find((item) => item.id === id)?.slug || id || "-"

  return (
    <div className="grid gap-4 xl:grid-cols-[minmax(0,0.9fr)_minmax(0,1.1fr)]">
      <Card>
        <CardHeader>
          <CardTitle>授权码</CardTitle>
          <CardDescription>授权码只显示一次，用户注册成功后自动失效。</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="invite-label">备注</FieldLabel>
              <Input id="invite-label" value={label} onChange={(event) => setLabel(event.target.value)} placeholder="例如 Alice" />
            </Field>
            <Button onClick={createInvite} disabled={loading}>
              {loading ? <Loader2Icon data-icon="inline-start" /> : <KeyRoundIcon data-icon="inline-start" />}
              创建授权码
            </Button>
          </FieldGroup>
          {createdCode ? (
            <Alert>
              <KeyRoundIcon />
              <AlertTitle>请现在复制授权码</AlertTitle>
              <AlertDescription className="flex flex-col gap-2">
                <span className="break-all font-mono text-sm">{createdCode}</span>
                <Button variant="outline" size="sm" className="w-fit" onClick={() => copyText(createdCode, "授权码已复制")}>
                  <ClipboardIcon data-icon="inline-start" />
                  复制授权码
                </Button>
              </AlertDescription>
            </Alert>
          ) : null}
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>备注</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>使用者</TableHead>
                  <TableHead>创建时间</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {inviteCodes.length === 0 ? (
                  <TableRow><TableCell colSpan={5} className="text-muted-foreground">暂无授权码</TableCell></TableRow>
                ) : inviteCodes.map((invite) => (
                  <TableRow key={invite.id}>
                    <TableCell className="max-w-40 truncate">{invite.label || "-"}</TableCell>
                    <TableCell><Badge variant={invite.usedAt ? "secondary" : "outline"}>{invite.usedAt ? "已使用" : "未使用"}</Badge></TableCell>
                    <TableCell className="max-w-40 truncate">{userName(invite.usedByUserId)}</TableCell>
                    <TableCell className="text-muted-foreground">{formatTime(invite.createdAt)}</TableCell>
                    <TableCell className="text-right">
                      <Button variant="ghost" size="sm" onClick={() => deleteInvite(invite)}>
                        <Trash2Icon data-icon="inline-start" />
                        删除
                      </Button>
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>用户</CardTitle>
          <CardDescription>admin 可切换到任意用户空间进行管理。</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>用户</TableHead>
                  <TableHead>角色</TableHead>
                  <TableHead>状态</TableHead>
                  <TableHead>最近登录</TableHead>
                  <TableHead className="text-right">操作</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {users.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell className="min-w-48">
                      <div className="flex flex-col">
                        <span className="font-medium">{item.name || item.slug}</span>
                        <span className="text-xs text-muted-foreground">/{item.slug}</span>
                      </div>
                    </TableCell>
                    <TableCell><Badge variant={item.role === "admin" ? "default" : "outline"}>{item.role}</Badge></TableCell>
                    <TableCell><Badge variant={item.enabled ? "secondary" : "destructive"}>{item.enabled ? "启用" : "禁用"}</Badge></TableCell>
                    <TableCell className="text-muted-foreground">{formatTime(item.lastLoginAt)}</TableCell>
                    <TableCell className="text-right">
                      {item.role === "admin" ? (
                        <Button variant="ghost" size="sm" disabled>固定启用</Button>
                      ) : (
                        <Button variant="outline" size="sm" onClick={() => setUserEnabled(item, !item.enabled)}>
                          {item.enabled ? "禁用" : "启用"}
                        </Button>
                      )}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          </div>
        </CardContent>
      </Card>
    </div>
  )
}

function SettingsView({
  api,
  settings,
  userToken,
  onAdminTokenUpdated,
  onUserTokenUpdated,
}: {
  api: API
  settings: Settings | null
  userToken: string
  onAdminTokenUpdated: (token: string) => void
  onUserTokenUpdated: (settings: Settings, userToken: string) => void
}) {
  const [currentAdminToken, setCurrentAdminToken] = useState("")
  const [newAdminToken, setNewAdminToken] = useState("")
  const [confirmAdminToken, setConfirmAdminToken] = useState("")
  const [userTokenDraft, setUserTokenDraft] = useState(userToken)
  const [refreshMinutesDraft, setRefreshMinutesDraft] = useState(String(settings?.refreshMinutes ?? 60))
  const [trafficMinutesDraft, setTrafficMinutesDraft] = useState(String(settings?.trafficQueryMinutes ?? 5))
  const [busy, setBusy] = useState("")

  useEffect(() => {
    setUserTokenDraft(userToken)
  }, [userToken])

  useEffect(() => {
    setTrafficMinutesDraft(String(settings?.trafficQueryMinutes ?? 5))
  }, [settings?.trafficQueryMinutes])

  useEffect(() => {
    setRefreshMinutesDraft(String(settings?.refreshMinutes ?? 60))
  }, [settings?.refreshMinutes])

  async function updateAdminToken() {
    if (newAdminToken !== confirmAdminToken) {
      toast.error("两次输入的新管理员 token 不一致")
      return
    }
    setBusy("admin-token")
    try {
      const response = await api.put<{ token: string }>("/api/settings/admin-token", {
        currentToken: currentAdminToken,
        newToken: newAdminToken,
      })
      onAdminTokenUpdated(response.token)
      setCurrentAdminToken("")
      setNewAdminToken("")
      setConfirmAdminToken("")
      toast.success("管理员 token 已修改")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  async function updateUserToken(nextToken = userTokenDraft.trim()) {
    setBusy("user-token")
    try {
      const nextSettings = await api.put<Settings>("/api/settings/user-token", { token: nextToken })
      onUserTokenUpdated(nextSettings, nextToken)
      setUserTokenDraft(nextToken)
      toast.success(nextToken ? "用户 token 已保存" : "用户 token 已清空")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  async function updateTrafficQuerySettings() {
    const minutes = Number.parseInt(trafficMinutesDraft, 10)
    if (!Number.isFinite(minutes) || minutes < 1 || minutes > 1440) {
      toast.error("流量查询间隔需要在 1 到 1440 分钟之间")
      return
    }
    setBusy("traffic-query")
    try {
      const nextSettings = await api.put<Settings>("/api/settings/traffic-query", { minutes })
      onUserTokenUpdated(nextSettings, userTokenDraft.trim())
      setTrafficMinutesDraft(String(nextSettings.trafficQueryMinutes))
      toast.success("流量查询设置已保存")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  async function updateRefreshSettings() {
    const minutes = Number.parseInt(refreshMinutesDraft, 10)
    if (!Number.isFinite(minutes) || minutes < 1 || minutes > 1440) {
      toast.error("自动刷新间隔需要在 1 到 1440 分钟之间")
      return
    }
    setBusy("refresh")
    try {
      const nextSettings = await api.put<Settings>("/api/settings/refresh", { minutes })
      onUserTokenUpdated(nextSettings, userTokenDraft.trim())
      setRefreshMinutesDraft(String(nextSettings.refreshMinutes))
      toast.success("自动刷新设置已保存")
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  const userTokenEnabled = settings?.hasUserToken ?? false
  const canSaveAdminToken = currentAdminToken.length >= 8 && newAdminToken.length >= 8 && confirmAdminToken.length >= 8
  const canSaveUserToken = userTokenDraft.trim().length >= 8
  const canSaveRefreshMinutes = Number.parseInt(refreshMinutesDraft, 10) >= 1 && Number.parseInt(refreshMinutesDraft, 10) <= 1440
  const canSaveTrafficMinutes = Number.parseInt(trafficMinutesDraft, 10) >= 1 && Number.parseInt(trafficMinutesDraft, 10) <= 1440

  return (
    <div className="grid gap-4 lg:grid-cols-2">
      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-3">
            <div>
              <CardTitle>管理员 token</CardTitle>
              <CardDescription>用于登录后台和管理配置，修改后当前浏览器会自动换成新会话。</CardDescription>
            </div>
            <Badge variant="secondary"><ShieldIcon />后台</Badge>
          </div>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="current-admin-token">当前管理员 token</FieldLabel>
              <PasswordInput
                id="current-admin-token"
                value={currentAdminToken}
                onChange={(event) => setCurrentAdminToken(event.target.value)}
                placeholder="输入当前 token"
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="new-admin-token">新管理员 token</FieldLabel>
              <PasswordInput
                id="new-admin-token"
                value={newAdminToken}
                onChange={(event) => setNewAdminToken(event.target.value)}
                placeholder="至少 8 位"
              />
            </Field>
            <Field>
              <FieldLabel htmlFor="confirm-admin-token">确认新 token</FieldLabel>
              <PasswordInput
                id="confirm-admin-token"
                value={confirmAdminToken}
                onChange={(event) => setConfirmAdminToken(event.target.value)}
                placeholder="再次输入新 token"
              />
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter>
          <Button disabled={busy === "admin-token" || !canSaveAdminToken} onClick={updateAdminToken}>
            {busy === "admin-token" ? <Loader2Icon data-icon="inline-start" /> : <SaveIcon data-icon="inline-start" />}
            修改管理员 token
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-3">
            <div>
              <CardTitle>公开订阅 token</CardTitle>
              <CardDescription>可选，用于给公开订阅地址增加访问 token。</CardDescription>
            </div>
            <Badge variant={userTokenEnabled ? "default" : "outline"}>
              <KeyRoundIcon />
              {userTokenEnabled ? "已启用" : "未启用"}
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="user-token">公开订阅用户 token</FieldLabel>
              <PasswordInput
                id="user-token"
                value={userTokenDraft}
                onChange={(event) => setUserTokenDraft(event.target.value)}
                placeholder={userTokenEnabled ? "输入新的用户 token" : "至少 8 位"}
              />
              <FieldDescription>
                {userTokenEnabled
                  ? userToken
                    ? "本浏览器已保存用户 token，复制订阅地址时会自动带上。"
                    : "服务端已启用用户 token；如需复制可用链接，请在这里重新输入并保存。"
                  : "未设置时公开订阅地址保持原来的直接访问方式。"}
              </FieldDescription>
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter className="flex flex-wrap gap-2">
          <Button disabled={busy === "user-token" || !canSaveUserToken} onClick={() => updateUserToken()}>
            {busy === "user-token" ? <Loader2Icon data-icon="inline-start" /> : <SaveIcon data-icon="inline-start" />}
            保存用户 token
          </Button>
          <Button variant="outline" disabled={busy === "user-token" || !userTokenEnabled} onClick={() => updateUserToken("")}>
            清空用户 token
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-3">
            <div>
              <CardTitle>订阅源自动刷新</CardTitle>
              <CardDescription>按固定频率自动执行顶部的“刷新全部”，更新节点缓存。</CardDescription>
            </div>
            <Badge variant="secondary">
              <RefreshCcwIcon />
              {settings?.refreshMinutes ?? 60} 分钟
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="refresh-minutes">刷新间隔（分钟）</FieldLabel>
              <Input
                id="refresh-minutes"
                type="number"
                min={1}
                max={1440}
                step={1}
                value={refreshMinutesDraft}
                onChange={(event) => setRefreshMinutesDraft(event.target.value)}
              />
              <FieldDescription>
                默认 60 分钟。后台只刷新已启用的订阅源；正在刷新中的源会自动跳过。
              </FieldDescription>
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter>
          <Button disabled={busy === "refresh" || !canSaveRefreshMinutes} onClick={updateRefreshSettings}>
            {busy === "refresh" ? <Loader2Icon data-icon="inline-start" /> : <SaveIcon data-icon="inline-start" />}
            保存自动刷新设置
          </Button>
        </CardFooter>
      </Card>

      <Card>
        <CardHeader>
          <div className="flex items-start justify-between gap-3">
            <div>
              <CardTitle>流量查询</CardTitle>
              <CardDescription>独立于节点刷新，定时更新公开订阅响应头里的用量。</CardDescription>
            </div>
            <Badge variant="secondary">
              <DatabaseBackupIcon />
              {settings?.trafficQueryMinutes ?? 5} 分钟
            </Badge>
          </div>
        </CardHeader>
        <CardContent>
          <FieldGroup>
            <Field>
              <FieldLabel htmlFor="traffic-query-minutes">查询间隔（分钟）</FieldLabel>
              <Input
                id="traffic-query-minutes"
                type="number"
                min={1}
                max={1440}
                step={1}
                value={trafficMinutesDraft}
                onChange={(event) => setTrafficMinutesDraft(event.target.value)}
              />
              <FieldDescription>
                默认 5 分钟。后台只更新流量信息，不刷新节点；公开订阅会立即使用最近一次成功查询结果。
              </FieldDescription>
            </Field>
          </FieldGroup>
        </CardContent>
        <CardFooter>
          <Button disabled={busy === "traffic-query" || !canSaveTrafficMinutes} onClick={updateTrafficQuerySettings}>
            {busy === "traffic-query" ? <Loader2Icon data-icon="inline-start" /> : <SaveIcon data-icon="inline-start" />}
            保存流量查询设置
          </Button>
        </CardFooter>
      </Card>
    </div>
  )
}

function RulesView({
  api,
  ruleSources,
  ruleSets,
  outputs,
  scopeQuery,
  reload,
}: {
  api: API
  ruleSources: RuleSource[]
  ruleSets: RuleSet[]
  outputs: Output[]
  scopeQuery: string
  reload: () => Promise<void>
}) {
  const [sourcesDraft, setSourcesDraft] = useState<RuleSource[]>([])
  const [setsDraft, setSetsDraft] = useState<RuleSet[]>([])
  const [activeSetId, setActiveSetId] = useState("")
  const [busy, setBusy] = useState("")

  useEffect(() => {
    const nextSources = ruleSources.length ? ruleSources : [defaultRuleSource]
    setSourcesDraft(nextSources.map(normalizeRuleSourceDraft))
  }, [ruleSources])

  useEffect(() => {
    const nextSets = ruleSets.length ? ruleSets : [defaultRuleSet]
    setSetsDraft(nextSets.map(normalizeRuleSetDraft))
    setActiveSetId((current) => current && nextSets.some((set) => set.id === current) ? current : nextSets[0]?.id ?? "")
  }, [ruleSets])

  const activeSet = setsDraft.find((set) => set.id === activeSetId) ?? setsDraft[0]
  const referencedOutputCount = outputs.filter((output) => (output.pac.ruleSetId || defaultOutput.pac.ruleSetId) === activeSet?.id).length
  const referencedOutputNames = outputs
    .filter((output) => (output.pac.ruleSetId || defaultOutput.pac.ruleSetId) === activeSet?.id)
    .map((output) => output.name)
  const selectedSources = sourcesDraft.filter((source) => activeSet?.sourceIds.includes(source.id))
  const selectedSourceDomainCount = selectedSources.reduce((total, source) => total + cachedDomainCount(source), 0)
  const activeCachedDomainCount = cachedDomainCount(activeSet) || selectedSourceDomainCount
  const keywordRuleCount = activeSet?.domainKeywords?.length ?? 0
  const manualDomainCount = activeSet?.directDomainSuffixes.length ?? 0
  const totalCommunityDomainCount = sourcesDraft.reduce((total, source) => total + cachedDomainCount(source), 0)
  const errorSourceCount = sourcesDraft.filter((source) => source.lastSyncStatus === "error").length
  const activeSetIsSaved = Boolean(activeSet && ruleSets.some((set) => set.id === activeSet.id))

  function updateSource(index: number, patch: Partial<RuleSource>) {
    const current = sourcesDraft[index]
    if (!current) {
      return
    }
    const nextSource = normalizeRuleSourceDraft({ ...current, ...patch })
    setSourcesDraft((items) => items.map((item, itemIndex) => itemIndex === index ? nextSource : item))
    if (nextSource.id !== current.id) {
      setSetsDraft((items) => items.map((set) => normalizeRuleSetDraft({
        ...set,
        sourceIds: set.sourceIds.map((id) => id === current.id ? nextSource.id : id),
      })))
    }
  }

  function updateActiveSet(patch: Partial<RuleSet>) {
    if (!activeSet) {
      return
    }
    const nextID = patch.id ? normalizeDraftID(patch.id) : activeSet.id
    setSetsDraft((items) => items.map((item) => item.id === activeSet.id ? normalizeRuleSetDraft({ ...item, ...patch }) : item))
    if (nextID !== activeSet.id) {
      setActiveSetId(nextID)
    }
  }

  function addSource() {
    const id = uniqueDraftID("rule-source", sourcesDraft.map((source) => source.id))
    const nextSource = normalizeRuleSourceDraft({
      ...defaultRuleSource,
      id,
      name: "新规则源",
      url: "",
      localPath: "",
      cachedDomainSuffixes: [],
      cachedDomainCount: 0,
    })
    setSourcesDraft([...sourcesDraft, normalizeRuleSourceDraft({
      ...nextSource,
    })])
    if (activeSet) {
      setSetsDraft((items) => items.map((set) => set.id === activeSet.id ? normalizeRuleSetDraft({
        ...set,
        sourceIds: [...set.sourceIds, nextSource.id],
      }) : set))
    }
  }

  function removeSource(id: string) {
    if (sourcesDraft.length <= 1) {
      return
    }
    const nextSources = sourcesDraft.filter((source) => source.id !== id)
    setSourcesDraft(nextSources)
    setSetsDraft((items) => items.map((set) => normalizeRuleSetDraft({
      ...set,
      sourceIds: set.sourceIds.filter((sourceId) => sourceId !== id),
    })))
  }

  function addSet() {
    const id = uniqueDraftID("custom-direct", setsDraft.map((set) => set.id))
    const nextSet = normalizeRuleSetDraft({
      ...defaultRuleSet,
      id,
      name: `自定义方案 ${setsDraft.length + 1}`,
      sourceIds: sourcesDraft.map((source) => source.id),
      cachedDomainSuffixes: [],
      cachedDomainCount: 0,
    })
    setSetsDraft([...setsDraft, nextSet])
    setActiveSetId(nextSet.id)
  }

  function removeSet(id: string) {
    if (setsDraft.length <= 1 || referencedOutputCount > 0) {
      return
    }
    const nextSets = setsDraft.filter((set) => set.id !== id)
    setSetsDraft(nextSets.length ? nextSets : [defaultRuleSet])
    setActiveSetId(nextSets[0]?.id ?? defaultRuleSet.id)
  }

  async function saveRules() {
    const nextSources = sourcesDraft.map(normalizeRuleSourceDraft)
    const sourceIDs = new Set(nextSources.map((source) => source.id).filter(Boolean))
    const nextSets = setsDraft
      .map(normalizeRuleSetDraft)
      .map((set) => ({ ...set, sourceIds: set.sourceIds.filter((id) => sourceIDs.has(id)) }))
    if (nextSources.length === 0 || nextSets.length === 0) {
      toast.error("至少保留一个规则源和一个规则集")
      return
    }
    if (nextSources.some((source) => !source.id || !source.name)) {
      toast.error("规则源 ID 和名称不能为空")
      return
    }
    if (nextSets.some((set) => !set.id || !set.name)) {
      toast.error("方案 ID 和名称不能为空")
      return
    }
    if (new Set(nextSources.map((source) => source.id)).size !== nextSources.length) {
      toast.error("规则源 ID 不能重复")
      return
    }
    if (new Set(nextSets.map((set) => set.id)).size !== nextSets.length) {
      toast.error("规则集 ID 不能重复")
      return
    }
    setBusy("save")
    try {
      await api.put(withScope("/api/rule-sources", scopeQuery), nextSources.map(ruleSourceSavePayload))
      await api.put(withScope("/api/rule-sets", scopeQuery), nextSets.map(ruleSetSavePayload))
      toast.success("规则配置已保存")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  async function syncActiveSet() {
    if (!activeSet) {
      return
    }
    if (!activeSetIsSaved) {
      toast.error("请先保存方案，再同步社区规则")
      return
    }
    setBusy("sync")
    try {
      await api.post(withScope(`/api/rule-sets/${activeSet.id}/sync`, scopeQuery), {})
      toast.success("规则集已同步")
      await reload()
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setBusy("")
    }
  }

  return (
    <div className="space-y-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <h2 className="text-lg font-semibold">规则</h2>
          <p className="text-sm text-muted-foreground">社区规则源独立维护；每个公开订阅只绑定一个直连方案。</p>
        </div>
        <Button disabled={busy !== ""} onClick={saveRules}>
          {busy === "save" ? <Loader2Icon data-icon="inline-start" /> : <SaveIcon data-icon="inline-start" />}
          保存全部
        </Button>
      </div>

      <Card>
        <CardHeader className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <CardTitle>社区规则源</CardTitle>
            <CardDescription>维护可复用的上游规则列表，方案只引用这些来源。</CardDescription>
          </div>
          <Button variant="outline" onClick={addSource}>
            <PlusIcon data-icon="inline-start" />
            新增规则源
          </Button>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="grid gap-3 md:grid-cols-3">
            <Metric label="规则源" value={sourcesDraft.length} />
            <Metric label="缓存域名" value={totalCommunityDomainCount} />
            <Metric label="异常来源" value={errorSourceCount || "-"} />
          </div>
          <div className="grid gap-3 lg:grid-cols-3">
            {sourcesDraft.map((source, index) => (
              <div key={`${source.id}-${index}`} className="rounded-md border bg-background p-3">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-medium">{source.name || "未命名规则源"}</span>
                      <Badge variant={source.lastSyncStatus === "error" ? "destructive" : "secondary"}>
                        {cachedDomainCount(source)} 条
                      </Badge>
                    </div>
                    <p className="mt-1 break-all text-xs text-muted-foreground">{source.url || source.localPath || "尚未配置来源"}</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      {source.lastSyncStatus === "error" ? `同步失败：${source.lastSyncError || "未知错误"}` : `最近同步：${formatTime(source.lastSyncedAt)}`}
                    </p>
                  </div>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon-sm"
                    disabled={sourcesDraft.length <= 1}
                    title="删除规则源"
                    onClick={() => removeSource(source.id)}
                  >
                    <Trash2Icon />
                  </Button>
                </div>

                <details className="mt-3 rounded-md bg-muted/25 px-3 py-2">
                  <summary className="cursor-pointer text-sm font-medium text-muted-foreground">编辑来源</summary>
                  <div className="mt-3 grid gap-3">
                    <div className="grid gap-3 sm:grid-cols-2">
                      <Field>
                        <FieldLabel>显示名称</FieldLabel>
                        <Input value={source.name} onChange={(event) => updateSource(index, { name: event.target.value })} />
                      </Field>
                      <Field>
                        <FieldLabel>来源 ID</FieldLabel>
                        <Input value={source.id} onChange={(event) => updateSource(index, { id: event.target.value })} />
                      </Field>
                    </div>
                    <Field>
                      <FieldLabel>文件格式</FieldLabel>
                      <Select value={source.format || "domain-list"} onValueChange={(value) => updateSource(index, { format: String(value) })}>
                        <SelectTrigger className="w-full"><SelectValue /></SelectTrigger>
                        <SelectContent>
                          <SelectGroup>
                            <SelectItem value="clash-domain">Clash DOMAIN</SelectItem>
                            <SelectItem value="dnsmasq">dnsmasq</SelectItem>
                            <SelectItem value="domain-list">纯域名列表</SelectItem>
                            <SelectItem value="yaml-payload">YAML payload</SelectItem>
                          </SelectGroup>
                        </SelectContent>
                      </Select>
                    </Field>
                    <Field>
                      <FieldLabel>刷新间隔（小时）</FieldLabel>
                      <Input type="number" min={1} value={source.refreshHours} onChange={(event) => updateSource(index, { refreshHours: Number.parseInt(event.target.value, 10) || 24 })} />
                    </Field>
                    <Field>
                      <FieldLabel>在线地址</FieldLabel>
                      <Input value={source.url} onChange={(event) => updateSource(index, { url: event.target.value })} placeholder="https://example.com/direct.list" />
                    </Field>
                    <Field>
                      <FieldLabel>本地文件</FieldLabel>
                      <Input value={source.localPath ?? ""} onChange={(event) => updateSource(index, { localPath: event.target.value })} placeholder="rules/pac/example.list" />
                    </Field>
                  </div>
                </details>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader className="flex flex-col gap-3 lg:flex-row lg:items-start lg:justify-between">
          <div>
            <CardTitle>直连方案</CardTitle>
            <CardDescription>创建自己的方案，公开订阅在这里选择一个方案使用。</CardDescription>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button variant="outline" onClick={addSet}>
              <PlusIcon data-icon="inline-start" />
              创建方案
            </Button>
            <Button variant="outline" disabled={!activeSet || busy !== ""} onClick={syncActiveSet}>
              {busy === "sync" ? <Loader2Icon data-icon="inline-start" /> : <RefreshCcwIcon data-icon="inline-start" />}
              同步当前方案
            </Button>
          </div>
        </CardHeader>
        <CardContent>
          {activeSet ? (
            <div className="grid gap-4 lg:grid-cols-[18rem_minmax(0,1fr)]">
              <div className="space-y-2">
                {setsDraft.map((set) => {
                  const setReferencedCount = outputs.filter((output) => (output.pac.ruleSetId || defaultOutput.pac.ruleSetId) === set.id).length
                  const selectedCount = set.sourceIds.filter((sourceId) => sourcesDraft.some((source) => source.id === sourceId)).length
                  return (
                    <button
                      key={set.id}
                      type="button"
                      className={cn(
                        "w-full rounded-md border p-3 text-left transition hover:bg-muted/40",
                        set.id === activeSet.id && "border-primary bg-muted/35",
                      )}
                      onClick={() => setActiveSetId(set.id)}
                    >
                      <span className="block truncate text-sm font-medium">{set.name || "未命名方案"}</span>
                      <span className="mt-1 block truncate text-xs text-muted-foreground">{set.id || "尚未设置 ID"}</span>
                      <span className="mt-2 flex flex-wrap gap-1.5">
                        <Badge variant="secondary">{selectedCount} 个来源</Badge>
                        <Badge variant="outline">{cachedDomainCount(set)} 条缓存</Badge>
                        {setReferencedCount ? <Badge>{setReferencedCount} 个订阅</Badge> : null}
                      </span>
                    </button>
                  )
                })}
              </div>

              <div className="space-y-4 rounded-lg border p-4">
                <div className="grid gap-3 md:grid-cols-5">
                  <Metric label="社区规则" value={selectedSources.length} />
                  <RuleDomainDialog
                    api={api}
                    ruleSet={activeSet}
                    scopeQuery={scopeQuery}
                    count={activeCachedDomainCount}
                    disabled={!activeSetIsSaved || activeCachedDomainCount === 0}
                  />
                  <Metric label="代理关键词" value={keywordRuleCount || "-"} />
                  <Metric label="手工直连" value={manualDomainCount || "-"} />
                  <Metric label="订阅引用" value={referencedOutputCount} />
                </div>

                <div className="grid gap-3 sm:grid-cols-2">
                  <Field>
                    <FieldLabel>方案名称</FieldLabel>
                    <Input value={activeSet.name} onChange={(event) => updateActiveSet({ name: event.target.value })} />
                  </Field>
                  <Field>
                    <FieldLabel>方案 ID</FieldLabel>
                    <Input value={activeSet.id} onChange={(event) => updateActiveSet({ id: event.target.value })} />
                  </Field>
                </div>

                <Field>
                  <div className="flex items-center justify-between gap-2">
                    <FieldLabel>选择社区规则源</FieldLabel>
                    <Badge variant="secondary">{selectedSources.length} / {sourcesDraft.length}</Badge>
                  </div>
                  <div className="grid gap-2 sm:grid-cols-2">
                    {sourcesDraft.map((source) => {
                      const selected = activeSet.sourceIds.includes(source.id)
                      return (
                        <label
                          key={source.id}
                          className={cn(
                            "flex items-start gap-3 rounded-md border p-3 text-sm",
                            selected && "border-primary/60 bg-muted/30",
                          )}
                        >
                          <Checkbox
                            checked={selected}
                            onCheckedChange={(checked) => {
                              const nextIDs = checked
                                ? [...activeSet.sourceIds, source.id]
                                : activeSet.sourceIds.filter((id) => id !== source.id)
                              updateActiveSet({ sourceIds: splitList(nextIDs.join(",")) })
                            }}
                          />
                          <span className="min-w-0">
                            <span className="block truncate font-medium">{source.name}</span>
                            <span className="mt-1 block text-xs text-muted-foreground">
                              {cachedDomainCount(source)} 条，{source.lastSyncStatus === "error" ? "同步失败" : `最近同步 ${formatTime(source.lastSyncedAt)}`}
                            </span>
                          </span>
                        </label>
                      )
                    })}
                  </div>
                  <FieldDescription>方案会合并已勾选的社区规则源，并应用下面的手工补充与排除名单。</FieldDescription>
                </Field>

                <div className="grid gap-4 xl:grid-cols-3">
                  <EditableRuleRows
                    label="总是代理的域名关键词"
                    placeholder="poe"
                    values={activeSet.domainKeywords ?? []}
                    onChange={(values) => updateActiveSet({ domainKeywords: values })}
                    emptyText="例如 poe，会生成 DOMAIN-KEYWORD 规则并交给节点选择。"
                  />
                  <EditableRuleRows
                    label="总是直连的网站"
                    placeholder="example.com"
                    values={activeSet.directDomainSuffixes}
                    onChange={(values) => updateActiveSet({ directDomainSuffixes: values })}
                    emptyText="例如公司内网、国内服务、你确认不需要代理的网站。"
                  />
                  <EditableRuleRows
                    label="不要直连的网站"
                    placeholder="blocked.example.com"
                    values={activeSet.excludedDomainSuffixes ?? []}
                    onChange={(values) => updateActiveSet({ excludedDomainSuffixes: values })}
                    emptyText="如果某个网站被社区规则误判为直连，在这里排除。"
                  />
                </div>

                <details className="rounded-lg border bg-muted/20 p-3">
                  <summary className="cursor-pointer text-sm font-medium">高级：直连 IP 段</summary>
                  <div className="mt-4">
                    <EditableRuleRows
                      label="直连 IP 段"
                      placeholder="10.0.0.0/8"
                      values={activeSet.directCidrs}
                      onChange={(values) => updateActiveSet({ directCidrs: values })}
                      emptyText="只有明确知道 CIDR 含义时再填写。"
                    />
                  </div>
                </details>

                <Alert>
                  <CheckCircle2Icon />
                  <AlertTitle>当前方案：{activeSet.name}</AlertTitle>
                  <AlertDescription>
                    公开订阅会选择这个方案，不直接选择社区规则源。
                    {referencedOutputCount ? ` 已被 ${referencedOutputNames.join("、")} 引用。` : " 还没有公开订阅引用。"}
                    {activeSet.lastSyncStatus === "error" && activeSet.lastSyncError ? ` 最近同步失败：${activeSet.lastSyncError}` : ""}
                  </AlertDescription>
                </Alert>

                <div className="flex flex-wrap items-center justify-between gap-3">
                  <p className="text-xs text-muted-foreground">
                    最近同步：{formatTime(activeSet.lastSyncedAt)}
                    {!activeSetIsSaved ? "。新方案保存后才能同步。" : ""}
                  </p>
                  <Button
                    type="button"
                    variant="ghost"
                    size="sm"
                    disabled={setsDraft.length <= 1 || referencedOutputCount > 0}
                    onClick={() => removeSet(activeSet.id)}
                  >
                    <Trash2Icon data-icon="inline-start" />
                    删除方案
                  </Button>
                </div>
              </div>
            </div>
          ) : (
            <Empty>
              <EmptyHeader>
                <EmptyMedia><FileTextIcon /></EmptyMedia>
                <EmptyTitle>暂无直连方案</EmptyTitle>
                <EmptyDescription>新建一个方案后即可选择社区规则源。</EmptyDescription>
              </EmptyHeader>
              <EmptyContent>
                <Button onClick={addSet}>
                  <PlusIcon data-icon="inline-start" />
                  新建方案
                </Button>
              </EmptyContent>
            </Empty>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function EditableRuleRows({
  label,
  placeholder,
  values,
  onChange,
  emptyText,
}: {
  label: string
  placeholder: string
  values: string[]
  onChange: (values: string[]) => void
  emptyText?: string
}) {
  const [query, setQuery] = useState("")
  const [newValue, setNewValue] = useState("")
  const rows = values
  const normalizedQuery = query.trim().toLowerCase()
  const visibleRows = rows
    .map((value, index) => ({ value, index }))
    .filter((row) => !normalizedQuery || row.value.toLowerCase().includes(normalizedQuery))

  function updateRow(index: number, value: string) {
    const next = rows.map((row, rowIndex) => rowIndex === index ? value : row)
    onChange(next)
  }

  function addRow(value = "") {
    const nextValue = value.trim()
    onChange(splitLines([...values, nextValue].join("\n")))
    setNewValue("")
  }

  function removeRow(index: number) {
    const next = rows.filter((_row, rowIndex) => rowIndex !== index)
    onChange(next)
  }

  function compactRows() {
    onChange(splitLines(rows.join("\n")))
  }

  return (
    <Field>
      <div className="flex items-center justify-between gap-2">
        <FieldLabel>{label}</FieldLabel>
        <Badge variant="secondary">{values.length} 条</Badge>
      </div>
      <div className="flex flex-col gap-2 sm:flex-row">
        <Input
          value={query}
          onChange={(event) => setQuery(event.target.value)}
          placeholder={`搜索${label}`}
        />
        <div className="flex gap-2 sm:min-w-56">
          <Input
            className="font-mono"
            value={newValue}
            onChange={(event) => setNewValue(event.target.value)}
            onKeyDown={(event) => {
              if (event.key === "Enter" && newValue.trim()) {
                event.preventDefault()
                addRow(newValue)
              }
            }}
            placeholder={placeholder}
          />
          <Button type="button" variant="outline" size="icon" onClick={() => addRow(newValue)} disabled={!newValue.trim()}>
            <PlusIcon />
          </Button>
        </div>
      </div>
      {rows.length || normalizedQuery ? (
        <div className="max-h-64 overflow-auto rounded-md border">
          {visibleRows.length ? visibleRows.map(({ value, index }) => (
          <div key={index} className="flex items-center gap-2 border-b p-2 last:border-b-0">
            <span className="w-10 shrink-0 text-center text-xs text-muted-foreground">{index + 1}</span>
            <Input
              className="h-9 font-mono text-sm"
              value={value}
              onBlur={compactRows}
              onChange={(event) => updateRow(index, event.target.value)}
              placeholder={placeholder}
            />
            <Button type="button" variant="ghost" size="icon" onClick={() => removeRow(index)}>
              <Trash2Icon />
            </Button>
          </div>
          )) : (
          <div className="flex items-center justify-between gap-3 p-3 text-sm text-muted-foreground">
            <span>没有匹配的规则</span>
            <Button type="button" variant="outline" size="sm" onClick={() => addRow(query)} disabled={!query.trim()}>
              <PlusIcon data-icon="inline-start" />
              添加搜索词
            </Button>
          </div>
          )}
        </div>
      ) : null}
      <FieldDescription>
        {normalizedQuery ? `显示 ${visibleRows.length} / ${values.length} 条，编辑后保存生效。` : values.length ? "可搜索、编辑、新增或删除，保存后生效。" : (emptyText ?? "暂无手工规则，可在右侧输入后添加。")}
      </FieldDescription>
    </Field>
  )
}

function RuleDomainDialog({
  api,
  ruleSet,
  scopeQuery,
  count,
  disabled,
}: {
  api: API
  ruleSet: RuleSet
  scopeQuery: string
  count: number
  disabled?: boolean
}) {
  const [open, setOpen] = useState(false)
  const [query, setQuery] = useState("")
  const [result, setResult] = useState<RuleDomainsView | null>(null)
  const [loading, setLoading] = useState(false)

  const loadDomains = useCallback(async (nextQuery: string) => {
    if (!ruleSet?.id) {
      return
    }
    setLoading(true)
    try {
      const params = new URLSearchParams({ limit: "500" })
      const trimmedQuery = nextQuery.trim()
      if (trimmedQuery) {
        params.set("q", trimmedQuery)
      }
      setResult(await api.get<RuleDomainsView>(
        withScope(`/api/rule-sets/${encodeURIComponent(ruleSet.id)}/domains?${params.toString()}`, scopeQuery),
      ))
    } catch (error) {
      toast.error(messageOf(error))
    } finally {
      setLoading(false)
    }
  }, [api, ruleSet?.id, scopeQuery])

  useEffect(() => {
    if (open) {
      void loadDomains("")
    }
  }, [loadDomains, open])

  useEffect(() => {
    if (!open) {
      setQuery("")
      setResult(null)
    }
  }, [open])

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <Metric
        label="缓存域名"
        value={count}
        actionLabel="搜索缓存域名"
        disabled={disabled}
        onClick={() => {
          if (!disabled) {
            setOpen(true)
          }
        }}
      />
      <DialogContent className="max-h-[86vh] overflow-y-auto sm:max-w-4xl">
        <DialogHeader>
          <DialogTitle>缓存域名</DialogTitle>
          <DialogDescription>
            {ruleSet.name} 的方案规则明细，包含缓存、代理关键词、手工直连和排除项，支持用域名、来源或类型搜索。
          </DialogDescription>
        </DialogHeader>
        <form
          className="flex flex-col gap-2 sm:flex-row"
          onSubmit={(event) => {
            event.preventDefault()
            void loadDomains(query)
          }}
        >
          <Input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="搜索域名或来源，例如 google source-a manual"
            className="font-mono"
          />
          <div className="flex gap-2 sm:w-auto">
            <Button type="submit" disabled={loading} className="flex-1 sm:flex-none">
              {loading ? <Loader2Icon data-icon="inline-start" /> : <SearchIcon data-icon="inline-start" />}
              查询
            </Button>
            <Button
              type="button"
              variant="outline"
              disabled={loading || !query.trim()}
              onClick={() => {
                setQuery("")
                void loadDomains("")
              }}
            >
              清空
            </Button>
          </div>
        </form>
        <div className="grid gap-3 sm:grid-cols-3">
          <Metric label="全部" value={result?.total ?? count} />
          <Metric label="匹配" value={result?.matched ?? "-"} />
          <Metric label="展示上限" value={result?.limit ?? 500} />
        </div>
        <div className="min-h-0 overflow-hidden rounded-md border">
          {loading && !result ? (
            <div className="p-3">
              <TableSkeleton />
            </div>
          ) : result?.domains.length ? (
            <div className="max-h-[48vh] overflow-auto">
              <Table>
                <TableHeader className="sticky top-0 z-10 bg-popover">
                  <TableRow>
                    <TableHead className="w-16">#</TableHead>
                    <TableHead>域名 / 关键词</TableHead>
                    <TableHead>来源</TableHead>
                    <TableHead>类型</TableHead>
                    <TableHead className="w-16 text-right">操作</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {result.domains.map((row, index) => (
                    <TableRow key={`${row.domain}-${row.source}-${row.type}-${index}`}>
                      <TableCell className="text-xs text-muted-foreground tabular-nums">{index + 1}</TableCell>
                      <TableCell className="min-w-56 font-mono">{row.domain}</TableCell>
                      <TableCell className="max-w-56 truncate text-muted-foreground">{row.source || "-"}</TableCell>
                      <TableCell>
                        <RuleDomainTypeBadge type={row.type} />
                      </TableCell>
                      <TableCell className="text-right">
                        <Button
                          type="button"
                          variant="ghost"
                          size="icon-sm"
                          title="复制域名"
                          onClick={() => copyText(row.domain)}
                        >
                          <ClipboardIcon />
                        </Button>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          ) : (
            <div className="flex min-h-40 items-center justify-center p-4 text-sm text-muted-foreground">
              {result ? "没有匹配的域名" : "打开后会加载缓存域名"}
            </div>
          )}
        </div>
        <p className="text-xs text-muted-foreground">
          {result?.truncated ? `结果较多，仅展示前 ${result.limit} 条。` : "查询表达式会匹配域名、来源和类型；排除项用于说明哪些缓存域名不会进入直连结果。"}
        </p>
      </DialogContent>
    </Dialog>
  )
}

function RuleDomainTypeBadge({ type }: { type: string }) {
  if (type === "manual") {
    return <Badge variant="secondary">手工直连</Badge>
  }
  if (type === "keyword") {
    return <Badge>代理关键词</Badge>
  }
  if (type === "excluded") {
    return <Badge variant="destructive">排除</Badge>
  }
  return <Badge variant="outline">缓存</Badge>
}

function Metric({
  label,
  value,
  actionLabel,
  disabled,
  onClick,
}: {
  label: string
  value: string | number
  actionLabel?: string
  disabled?: boolean
  onClick?: () => void
}) {
  const interactive = Boolean(onClick)
  const content = (
    <>
      <span className="text-xs text-muted-foreground">{label}</span>
      <span className="mt-1 flex items-center justify-between gap-2">
        <span className="truncate text-lg font-semibold">{value}</span>
        {actionLabel && !disabled ? <SearchIcon className="size-4 text-muted-foreground" /> : null}
      </span>
      {actionLabel ? <span className="mt-1 text-xs text-muted-foreground">{disabled ? "暂无可搜索缓存" : actionLabel}</span> : null}
    </>
  )
  if (interactive) {
    return (
      <button
        type="button"
        className={cn(
          "rounded-lg border p-3 text-left transition",
          disabled ? "cursor-not-allowed opacity-60" : "hover:bg-muted/35 focus-visible:ring-[3px] focus-visible:ring-ring/50",
        )}
        disabled={disabled}
        onClick={onClick}
      >
        {content}
      </button>
    )
  }
  return (
    <div className="rounded-lg border p-3">
      {content}
    </div>
  )
}

function OutputNodeNames({ names }: { names: string[] }) {
  return (
    <div className="rounded-lg border bg-muted/20 p-3">
      <div className="mb-2 flex items-center justify-between gap-3">
        <p className="text-xs font-medium text-muted-foreground">节点名称</p>
        <span className="text-xs text-muted-foreground">{names.length} 个</span>
      </div>
      {names.length === 0 ? (
        <p className="text-sm text-muted-foreground">暂无可用节点</p>
      ) : (
        <div className="flex max-h-24 flex-wrap gap-1.5 overflow-y-auto pr-1">
          {names.map((name, index) => (
            <span
              key={`${name}-${index}`}
              className="max-w-full truncate rounded-md border bg-background px-2 py-1 text-xs leading-none"
              title={name}
            >
              {name}
            </span>
          ))}
        </div>
      )}
    </div>
  )
}

function StatusBadge({ status }: { status: string }) {
  if (status === "ok") {
    return <Badge><CheckCircle2Icon />正常</Badge>
  }
  if (status === "refreshing") {
    return <Badge variant="secondary"><Loader2Icon />刷新中</Badge>
  }
  if (status === "paused") {
    return <Badge variant="outline"><PauseCircleIcon />暂停</Badge>
  }
  if (status === "error") {
    return <Badge variant="destructive"><WifiOffIcon />异常</Badge>
  }
  return <Badge variant="secondary">待刷新</Badge>
}

function LoadingShell() {
  return (
    <main className="mx-auto flex min-h-screen w-full max-w-5xl flex-col gap-4 p-6">
      <Skeleton className="h-12 w-72" />
      <div className="grid gap-3 sm:grid-cols-4">
        {Array.from({ length: 4 }).map((_, index) => <Skeleton key={index} className="h-32" />)}
      </div>
      <Skeleton className="h-96" />
    </main>
  )
}

function TableSkeleton() {
  return (
    <div className="flex flex-col gap-2">
      {Array.from({ length: 5 }).map((_, index) => <Skeleton key={index} className="h-12" />)}
    </div>
  )
}

function createAPI(token: string) {
  return {
    token,
    get<T>(path: string) {
      return request<T>(path, { method: "GET" }, token)
    },
    post<T>(path: string, body: unknown) {
      return request<T>(path, { method: "POST", body: JSON.stringify(body) }, token)
    },
    put<T>(path: string, body: unknown) {
      return request<T>(path, { method: "PUT", body: JSON.stringify(body) }, token)
    },
    delete<T>(path: string) {
      return request<T>(path, { method: "DELETE" }, token)
    },
  }
}

type API = ReturnType<typeof createAPI>

function readStoredUser(): User | null {
  const value = localStorage.getItem("sub-nest-user")
  if (!value) {
    return null
  }
  try {
    return JSON.parse(value) as User
  } catch {
    localStorage.removeItem("sub-nest-user")
    return null
  }
}

function withScope(path: string, scopeQuery: string) {
  if (!scopeQuery) {
    return path
  }
  const [base, query = ""] = path.split("?")
  const scope = scopeQuery.replace(/^\?/, "")
  return `${base}?${[query, scope].filter(Boolean).join("&")}`
}

async function request<T>(path: string, init: RequestInit, token: string): Promise<T> {
  const response = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
      ...(init.headers ?? {}),
    },
  })
  if (response.status === 204) {
    return undefined as T
  }
  const data = await response.json().catch(() => ({}))
  if (!response.ok) {
    throw new Error(data.error ?? "请求失败")
  }
  return data as T
}

function splitList(value: string) {
	return value.split(/[,，\n]/).map((item) => item.trim()).filter(Boolean)
}

function splitLines(value: string) {
	return splitList(value)
}

function parseRenameRules(value: string) {
	return value.split("\n").map((line) => {
		const [pattern, ...rest] = line.split("=>")
		return { pattern: pattern?.trim() ?? "", replacement: rest.join("=>").trim() }
	}).filter((rule) => rule.pattern)
}

function normalizeOutputDraft(output: Output): Output {
  const defaults = defaultOutput.pac
  return {
    ...output,
    pac: {
      ...defaults,
      ...(output.pac ?? {}),
      ruleSetId: output.pac?.ruleSetId ?? defaults.ruleSetId,
      domainKeywords: output.pac?.domainKeywords ?? defaults.domainKeywords,
      directDomainSuffixes: output.pac?.directDomainSuffixes ?? defaults.directDomainSuffixes,
      directCidrs: output.pac?.directCidrs ?? defaults.directCidrs,
      cachedDomainSuffixes: output.pac?.cachedDomainSuffixes ?? [],
    },
  }
}

function normalizeRuleSourceDraft(source: RuleSource): RuleSource {
  const id = normalizeDraftID(source.id || source.name)
  return {
    ...source,
    id,
    name: source.name?.trim() || id || "规则源",
    url: source.url?.trim() || "",
    format: source.format || "domain-list",
    refreshHours: Number.isFinite(source.refreshHours) && source.refreshHours > 0 ? source.refreshHours : 24,
    localPath: source.localPath?.trim() || "",
    cachedDomainSuffixes: splitList((source.cachedDomainSuffixes ?? []).join(",")),
    cachedDomainCount: cachedDomainCount(source),
  }
}

function normalizeRuleSetDraft(set: RuleSet): RuleSet {
  const id = normalizeDraftID(set.id || set.name)
  return {
    ...set,
    id,
    name: set.name?.trim() || id || "规则集",
    sourceIds: splitList((set.sourceIds ?? []).join(",")),
    domainKeywords: splitList((set.domainKeywords ?? []).join(",")),
    directDomainSuffixes: splitList((set.directDomainSuffixes ?? []).join(",")),
    excludedDomainSuffixes: splitList((set.excludedDomainSuffixes ?? []).join(",")),
    directCidrs: splitList((set.directCidrs ?? []).join(",")),
    cachedDomainSuffixes: splitList((set.cachedDomainSuffixes ?? []).join(",")),
    cachedDomainCount: cachedDomainCount(set),
  }
}

function ruleSourceSavePayload(source: RuleSource) {
  return {
    id: source.id,
    name: source.name,
    url: source.url,
    format: source.format,
    refreshHours: source.refreshHours,
    localPath: source.localPath ?? "",
  }
}

function ruleSetSavePayload(set: RuleSet) {
  return {
    id: set.id,
    name: set.name,
    sourceIds: set.sourceIds,
    domainKeywords: set.domainKeywords ?? [],
    directDomainSuffixes: set.directDomainSuffixes,
    excludedDomainSuffixes: set.excludedDomainSuffixes ?? [],
    directCidrs: set.directCidrs,
  }
}

function cachedDomainCount(item?: { cachedDomainCount?: number; cachedDomainSuffixes?: string[] }) {
  return item?.cachedDomainCount ?? item?.cachedDomainSuffixes?.length ?? 0
}

function normalizeDraftID(value: string) {
  return value
    .trim()
    .toLowerCase()
    .replace(/[\s_/\\]+/g, "-")
    .replace(/[^a-z0-9-]/g, "")
    .replace(/^-+|-+$/g, "")
}

function uniqueDraftID(prefix: string, ids: string[]) {
  const used = new Set(ids)
  let index = ids.length + 1
  let id = `${prefix}-${index}`
  while (used.has(id)) {
    index += 1
    id = `${prefix}-${index}`
  }
  return id
}

function formatTime(value?: string) {
  if (!value) {
    return "尚未刷新"
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(new Date(value))
}

function formatDate(value?: string) {
  if (!value) {
    return "-"
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(new Date(value))
}

function formatBytes(value?: number) {
  const n = Number(value ?? 0)
  if (!Number.isFinite(n) || n <= 0) {
    return "-"
  }
  const units = ["B", "KB", "MB", "GB", "TB", "PB"]
  let size = n
  let index = 0
  while (size >= 1024 && index < units.length - 1) {
    size /= 1024
    index += 1
  }
  return `${size >= 10 || index === 0 ? size.toFixed(0) : size.toFixed(1)} ${units[index]}`
}

function formatDelay(value?: number) {
  const n = Number(value ?? 0)
  if (!Number.isFinite(n) || n <= 0) {
    return "-"
  }
  return `${Math.round(n)}ms`
}

function headersToText(headers?: Record<string, string>) {
  return Object.entries(headers ?? {}).map(([key, value]) => `${key}: ${value}`).join("\n")
}

function sourceInputReady(source: Source) {
  if (!source.name.trim()) {
    return false
  }
  if ((source.sourceType ?? "url") === "file") {
    return Boolean(source.fileContent?.trim())
  }
  return Boolean(source.url?.trim())
}

function trafficStatusTitle(info?: TrafficInfo) {
  if (info?.lastStatus === "ok") {
    return "最近查询成功"
  }
  if (info?.lastStatus === "error") {
    return "最近查询失败"
  }
  return "尚未查询流量"
}

function trafficInfoText(info?: TrafficInfo) {
  if (!info || (!info.remainingBytes && !info.totalBytes && !info.uploadBytes && !info.downloadBytes && !info.expireAt)) {
    return "保存配置后可手动测试，刷新订阅源时也会自动查询。"
  }
  const parts = [
    info.remainingBytes ? `剩余 ${formatBytes(info.remainingBytes)}` : "",
    info.totalBytes ? `总量 ${formatBytes(info.totalBytes)}` : "",
    info.uploadBytes || info.downloadBytes ? `已用 ${formatBytes((info.uploadBytes ?? 0) + (info.downloadBytes ?? 0))}` : "",
    info.expireAt ? `到期 ${formatDate(info.expireAt)}` : "",
  ].filter(Boolean)
  return parts.join("，")
}

function TrafficDebugPanel({ info }: { info?: TrafficInfo }) {
  const debug = info?.debug
  if (!debug && !info?.lastError) {
    return null
  }
  const meta = [
    debug?.method && debug.url ? `${debug.method} ${debug.url}` : "",
    debug?.method && !debug.url ? `${debug.method} 未填写 URL` : "",
    debug?.status ? `HTTP ${debug.status}` : "",
    debug?.contentType ? `Content-Type ${debug.contentType}` : "",
    debug?.parserType ? `解析 ${debug.parserType}` : "",
  ].filter(Boolean)
  return (
    <Dialog>
      <DialogTrigger render={<Button type="button" variant="outline" size="sm" className="mt-2 w-fit" />}>
        <BugIcon data-icon="inline-start" />
        查看调试详情
        {debug?.statusCode ? <span className="text-muted-foreground">HTTP {debug.statusCode}</span> : null}
      </DialogTrigger>
      <DialogContent className="max-h-[86vh] overflow-y-auto sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>流量查询调试详情</DialogTitle>
          <DialogDescription>请求响应、解析路径命中情况和响应片段。</DialogDescription>
        </DialogHeader>
        <div className="grid gap-4 text-sm">
          {info?.lastError ? (
            <section className="grid gap-1">
              <h4 className="text-sm font-medium">错误原因</h4>
              <div className="break-words rounded-md border border-destructive/30 bg-destructive/5 p-2 text-destructive">
                {info.lastError}
              </div>
            </section>
          ) : null}
          {meta.length ? (
            <section className="grid gap-1">
              <h4 className="text-sm font-medium">请求</h4>
              <div className="break-words rounded-md border bg-muted/35 p-2 font-mono text-xs text-muted-foreground">
                {meta.join("\n")}
              </div>
            </section>
          ) : null}
          {debug?.paths?.length ? (
            <section className="grid gap-2">
              <h4 className="text-sm font-medium">解析路径</h4>
              <div className="grid gap-1">
                {debug.paths.map((item, index) => (
                  <div key={`${item.label}-${item.path}-${index}`} className="grid gap-1 rounded-md border p-2 sm:grid-cols-[4rem_1fr] sm:gap-3">
                    <Badge className="w-fit" variant={item.found ? "secondary" : "destructive"}>
                      {item.found ? "命中" : "未命中"}
                    </Badge>
                    <div className="min-w-0 break-words font-mono text-xs">
                      <div className="text-foreground">{item.label} {item.path}</div>
                      {item.value ? <div className="text-muted-foreground">值：{item.value}</div> : null}
                      {item.error ? <div className="text-destructive">错误：{item.error}</div> : null}
                    </div>
                  </div>
                ))}
              </div>
            </section>
          ) : null}
          {debug?.header ? (
            <section className="grid gap-1">
              <h4 className="text-sm font-medium">Subscription-Userinfo</h4>
              <div className="break-words rounded-md border bg-muted/35 p-2 font-mono text-xs text-muted-foreground">
                {debug.header}
              </div>
            </section>
          ) : null}
          {debug?.bodyPreview ? (
            <section className="grid gap-1">
              <h4 className="text-sm font-medium">响应片段</h4>
              <pre className="max-h-80 overflow-auto whitespace-pre-wrap break-words rounded-md border bg-muted/35 p-3 font-mono text-xs leading-relaxed text-muted-foreground">
                {debug.bodyPreview}
              </pre>
            </section>
          ) : null}
        </div>
      </DialogContent>
    </Dialog>
  )
}

async function copyText(value: string, message = "已复制") {
  try {
    if (navigator.clipboard?.writeText) {
      await navigator.clipboard.writeText(value)
    } else {
      fallbackCopyText(value)
    }
    toast.success(message)
  } catch (error) {
    try {
      fallbackCopyText(value)
      toast.success(message)
    } catch {
      toast.error(messageOf(error))
    }
  }
}

function subscriptionURL(publicExampleUrl: string | undefined, slug: string, userToken = "") {
  let url: URL
  try {
    url = new URL(publicExampleUrl || "/s/main", window.location.origin)
  } catch {
    url = new URL("/s/main", window.location.origin)
  }
  const browserOrigin = new URL(window.location.origin)
  url.protocol = browserOrigin.protocol
  url.host = browserOrigin.host
  url.pathname = url.pathname.replace(/\/s\/[^/]*$/, `/s/${slug}`)
  url.search = ""
  const token = userToken.trim()
  if (token) {
    url.searchParams.set("token", token)
  }
  return url.toString()
}

function pacURL(subscriptionUrl: string) {
  const url = new URL(subscriptionUrl, window.location.origin)
  url.pathname = url.pathname.replace(/\/s\/([^/]*)$/, (_match, slug) => `/s/${slug}.pac`)
  return url.toString()
}

function downloadSubscription(baseURL: string, slug: string, format: string) {
  const item = downloadFormatItems.find((candidate) => candidate.value === format)
  const url = new URL(baseURL, window.location.origin)
  url.searchParams.set("format", format)
  url.searchParams.set("download", "1")

  const link = document.createElement("a")
  link.href = url.toString()
  link.download = `${slug || "subscription"}-${item?.filename ?? "config.yaml"}`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  toast.success(`${item?.label ?? "配置文件"} 已开始下载`)
}

function downloadPAC(baseURL: string, slug: string) {
  const url = new URL(baseURL, window.location.origin)
  url.searchParams.set("download", "1")

  const link = document.createElement("a")
  link.href = url.toString()
  link.download = `${slug || "subscription"}.pac`
  document.body.appendChild(link)
  link.click()
  document.body.removeChild(link)
  toast.success("PAC 文件已开始下载")
}

function fallbackCopyText(value: string) {
  const textarea = document.createElement("textarea")
  textarea.value = value
  textarea.setAttribute("readonly", "")
  textarea.style.position = "fixed"
  textarea.style.top = "0"
  textarea.style.opacity = "0"
  document.body.appendChild(textarea)
  textarea.select()
  const copied = document.execCommand("copy")
  document.body.removeChild(textarea)
  if (!copied) {
    throw new Error("复制失败，请手动复制")
  }
}

function messageOf(error: unknown) {
  if (!(error instanceof Error)) {
    return "操作失败"
  }
  if (error.message === "Failed to fetch") {
    return "后端服务未运行或连接已断开，请确认 Go 服务正在监听 8080"
  }
  return error.message
}

export default App
