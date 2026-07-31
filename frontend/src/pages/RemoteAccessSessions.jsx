import { useQuery } from "@tanstack/react-query";
import { FitAddon } from "@xterm/addon-fit";
import { Terminal } from "@xterm/xterm";
import "@xterm/xterm/css/xterm.css";
import {
	AlertCircle,
	CheckCircle2,
	Clock,
	Download,
	MonitorUp,
	Pause,
	Play,
	RefreshCw,
	RotateCcw,
	Search,
	ShieldCheck,
	Timer,
	X,
	XCircle,
} from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { createPortal } from "react-dom";
import { remoteAccessAPI } from "../utils/api";

const statusStyles = {
	connecting:
		"bg-blue-100 text-blue-800 dark:bg-blue-950 dark:text-blue-200",
	connected:
		"bg-green-100 text-green-800 dark:bg-green-950 dark:text-green-200",
	closed: "bg-slate-100 text-slate-800 dark:bg-slate-800 dark:text-slate-200",
	failed: "bg-red-100 text-red-800 dark:bg-red-950 dark:text-red-200",
	timeout:
		"bg-amber-100 text-amber-800 dark:bg-amber-950 dark:text-amber-200",
	agent_disconnected:
		"bg-orange-100 text-orange-800 dark:bg-orange-950 dark:text-orange-200",
};

const statusIcons = {
	connecting: Clock,
	connected: CheckCircle2,
	closed: ShieldCheck,
	failed: XCircle,
	timeout: Timer,
	agent_disconnected: AlertCircle,
};

const formatDateTime = (value) => {
	if (!value) return "—";
	return new Intl.DateTimeFormat(undefined, {
		year: "numeric",
		month: "2-digit",
		day: "2-digit",
		hour: "2-digit",
		minute: "2-digit",
		second: "2-digit",
	}).format(new Date(value));
};

const formatDuration = (seconds) => {
	if (seconds === null || seconds === undefined) return "—";
	if (seconds < 60) return `${seconds}s`;
	const minutes = Math.floor(seconds / 60);
	const rest = seconds % 60;
	if (minutes < 60) return `${minutes}m ${rest}s`;
	const hours = Math.floor(minutes / 60);
	return `${hours}h ${minutes % 60}m`;
};

const sessionDurationSeconds = (session, now) => {
	if (Number.isFinite(session.duration_seconds)) {
		return session.duration_seconds;
	}
	if (!session.started_at) return null;

	const startMs = new Date(session.started_at).getTime();
	if (!Number.isFinite(startMs)) return null;

	const isLive = ["connected", "connecting"].includes(session.status);
	if (!isLive && !session.ended_at) return null;

	const endMs = isLive ? now : new Date(session.ended_at).getTime();
	if (!Number.isFinite(endMs) || endMs < startMs) return null;

	return Math.floor((endMs - startMs) / 1000);
};

const StatusBadge = ({ status }) => {
	const Icon = statusIcons[status] || AlertCircle;
	return (
		<span
			className={`inline-flex items-center gap-1 rounded-md px-2 py-1 text-xs font-medium ${statusStyles[status] || statusStyles.closed}`}
		>
			<Icon className="h-3.5 w-3.5" />
			{status.replaceAll("_", " ")}
		</span>
	);
};

const cleanTypescript = (text) =>
	text
		.replace(/^\[BEGIN TYPESCRIPT\]\r?\n?/, "")
		.replace(/\r?\n?\[END TYPESCRIPT\]\r?\n?$/, "");

const parseTiming = (text) =>
	text
		.split(/\r?\n/)
		.map((line) => line.trim())
		.filter(Boolean)
		.map((line) => {
			const [delay, size] = line.split(/\s+/);
			return {
				delay: Number.parseFloat(delay) || 0,
				size: Number.parseInt(size, 10) || 0,
			};
		})
		.filter((item) => item.size > 0);

const buildPlaybackChunks = (recordingBuffer, timingText) => {
	const decoder = new TextDecoder();
	const encoder = new TextEncoder();
	const cleanText = cleanTypescript(decoder.decode(recordingBuffer));
	const bytes = encoder.encode(cleanText);
	const timing = parseTiming(timingText);
	let offset = 0;
	const chunks = timing.map(({ delay, size }) => {
		const next = Math.min(offset + size, bytes.length);
		const data = decoder.decode(bytes.slice(offset, next));
		offset = next;
		return { delay, data };
	});
	if (offset < bytes.length) {
		chunks.push({ delay: 0, data: decoder.decode(bytes.slice(offset)) });
	}
	return chunks;
};

const RecordingPlayer = ({ session, onClose }) => {
	const terminalRef = useRef(null);
	const terminalInstanceRef = useRef(null);
	const fitAddonRef = useRef(null);
	const timersRef = useRef([]);
	const indexRef = useRef(0);
	const chunksRef = useRef([]);
	const [isPlaying, setIsPlaying] = useState(false);
	const [speed, setSpeed] = useState(1);
	const [terminalReady, setTerminalReady] = useState(false);

	const {
		data: recording,
		isLoading,
		error,
	} = useQuery({
		queryKey: ["remote-access-recording", session?.id],
		enabled: Boolean(session?.id),
		queryFn: async () => {
			const [meta, data, timing] = await Promise.all([
				remoteAccessAPI.getRecording(session.id).then((r) => r.data),
				remoteAccessAPI.getRecordingData(session.id).then((r) => r.data),
				remoteAccessAPI.getRecordingTiming(session.id).then((r) => r.data),
			]);
			return {
				meta,
				chunks: buildPlaybackChunks(data, timing),
			};
		},
	});

	const clearTimers = useCallback(() => {
		for (const timer of timersRef.current) {
			clearTimeout(timer);
		}
		timersRef.current = [];
	}, []);

	const reset = useCallback(() => {
		clearTimers();
		indexRef.current = 0;
		terminalInstanceRef.current?.reset();
		setIsPlaying(false);
	}, [clearTimers]);

	const playFromCurrent = useCallback(() => {
		if (!terminalInstanceRef.current || !chunksRef.current.length) return;
		clearTimers();
		setIsPlaying(true);

		const playNext = () => {
			const chunk = chunksRef.current[indexRef.current];
			if (!chunk) {
				setIsPlaying(false);
				return;
			}
			const delayMs = Math.max(0, (chunk.delay * 1000) / speed);
			const timer = setTimeout(() => {
				terminalInstanceRef.current?.write(chunk.data);
				indexRef.current += 1;
				playNext();
			}, delayMs);
			timersRef.current.push(timer);
		};

		playNext();
	}, [clearTimers, speed]);

	useEffect(() => {
		if (!terminalRef.current) return;
		const term = new Terminal({
			cursorBlink: false,
			fontFamily: '"Courier New", monospace',
			fontSize: 14,
			theme: {
				background: "#020617",
				foreground: "#e5e7eb",
				cursor: "#e5e7eb",
			},
		});
		const fitAddon = new FitAddon();
		term.loadAddon(fitAddon);
		term.open(terminalRef.current);
		fitAddon.fit();
		terminalInstanceRef.current = term;
		fitAddonRef.current = fitAddon;
		setTerminalReady(true);

		const onResize = () => fitAddon.fit();
		window.addEventListener("resize", onResize);
		return () => {
			window.removeEventListener("resize", onResize);
			clearTimers();
			term.dispose();
			terminalInstanceRef.current = null;
			setTerminalReady(false);
		};
	}, [clearTimers]);

	useEffect(() => {
		if (!terminalReady) return;
		chunksRef.current = recording?.chunks || [];
		reset();
		if (recording?.chunks?.length) {
			setTimeout(() => playFromCurrent(), 50);
		}
	}, [recording, reset, playFromCurrent, terminalReady]);

	const togglePlayback = () => {
		if (isPlaying) {
			clearTimers();
			setIsPlaying(false);
		} else {
			playFromCurrent();
		}
	};

	const modal = (
		<div className="fixed inset-0 z-[10000] flex items-center justify-center bg-black/80 p-3 sm:p-4">
			<div className="flex h-[88vh] w-full max-w-[calc(100vw-1.5rem)] flex-col overflow-hidden rounded-lg border border-secondary-800 bg-secondary-950 shadow-2xl sm:max-w-[calc(100vw-2rem)] xl:max-w-6xl">
				<div className="flex flex-col gap-3 border-b border-secondary-800 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
					<div className="min-w-0">
						<h2 className="truncate text-base font-semibold text-white">
							SSH Recording - {session.host_name || session.host_id}
						</h2>
						<p className="text-xs text-secondary-400">
							{session.username} · {formatDateTime(session.started_at)}
						</p>
					</div>
					<div className="flex flex-wrap items-center gap-2">
						<select
							value={speed}
							onChange={(e) => setSpeed(Number.parseFloat(e.target.value))}
							className="h-8 rounded-md border border-secondary-700 bg-secondary-900 px-2 text-xs text-white"
						>
							<option value={0.5}>0.5x</option>
							<option value={1}>1x</option>
							<option value={2}>2x</option>
							<option value={4}>4x</option>
						</select>
						<button
							type="button"
							onClick={reset}
							className="inline-flex h-8 items-center gap-1 rounded-md border border-secondary-700 px-2 text-xs text-secondary-200 hover:bg-secondary-800"
						>
							<RotateCcw className="h-3.5 w-3.5" />
							Restart
						</button>
						<button
							type="button"
							onClick={togglePlayback}
							disabled={!recording?.chunks?.length}
							className="inline-flex h-8 items-center gap-1 rounded-md bg-primary-600 px-3 text-xs font-medium text-white hover:bg-primary-700 disabled:opacity-50"
						>
							{isPlaying ? (
								<Pause className="h-3.5 w-3.5" />
							) : (
								<Play className="h-3.5 w-3.5" />
							)}
							{isPlaying ? "Pause" : "Play"}
						</button>
						<a
							href={remoteAccessAPI.downloadRecordingUrl(session.id)}
							className="inline-flex h-8 items-center gap-1 rounded-md border border-secondary-700 px-2 text-xs text-secondary-200 hover:bg-secondary-800"
						>
							<Download className="h-3.5 w-3.5" />
							Download
						</a>
						<button
							type="button"
							onClick={onClose}
							className="rounded-md p-1 text-secondary-300 hover:bg-secondary-800 hover:text-white"
						>
							<X className="h-5 w-5" />
						</button>
					</div>
				</div>
				{error ? (
					<div className="p-4 text-sm text-red-300">
						{error.response?.data?.error || "Failed to load recording"}
					</div>
				) : null}
				{isLoading ? (
					<div className="p-4 text-sm text-secondary-300">
						Loading recording...
					</div>
				) : null}
				{!isLoading && !error && recording?.chunks?.length === 0 ? (
					<div className="p-4 text-sm text-secondary-300">
						Recording is empty.
					</div>
				) : null}
				<div className="min-h-0 flex-1 p-4">
					<div ref={terminalRef} className="h-full w-full rounded bg-black" />
				</div>
			</div>
		</div>
	);
	return createPortal(modal, document.body);
};

const RemoteAccessSessions = () => {
	const [filters, setFilters] = useState({
		search: "",
		protocol: "",
		status: "",
	});
	const [selectedRecording, setSelectedRecording] = useState(null);
	const [now, setNow] = useState(() => Date.now());

	const queryParams = useMemo(
		() => ({
			limit: 100,
			search: filters.search || undefined,
			protocol: filters.protocol || undefined,
			status: filters.status || undefined,
		}),
		[filters],
	);

	const { data, isLoading, isFetching, refetch } = useQuery({
		queryKey: ["remote-access-sessions", queryParams],
		queryFn: () => remoteAccessAPI.listSessions(queryParams).then((r) => r.data),
		refetchInterval: 30000,
	});

	const sessions = data?.sessions || [];
	const hasLiveSessions = sessions.some((session) =>
		["connected", "connecting"].includes(session.status),
	);

	useEffect(() => {
		if (!hasLiveSessions) return;
		const interval = window.setInterval(() => setNow(Date.now()), 1000);
		return () => window.clearInterval(interval);
	}, [hasLiveSessions]);

	return (
		<div className="space-y-5">
			<div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
				<div>
					<h1 className="text-2xl font-semibold text-secondary-900 dark:text-white">
						Remote Access
					</h1>
					<p className="mt-1 text-sm text-secondary-600 dark:text-secondary-400">
						Session audit trail for SSH and RDP access.
					</p>
				</div>
				<button
					type="button"
					onClick={() => refetch()}
					className="inline-flex h-9 items-center gap-2 rounded-md border border-secondary-300 bg-white px-3 text-sm font-medium text-secondary-700 hover:bg-secondary-50 dark:border-secondary-700 dark:bg-secondary-900 dark:text-secondary-200 dark:hover:bg-secondary-800"
				>
					<RefreshCw
						className={`h-4 w-4 ${isFetching ? "animate-spin" : ""}`}
					/>
					Refresh
				</button>
			</div>

			<div className="grid gap-3 md:grid-cols-[1fr_180px_220px]">
				<label className="relative block">
					<Search className="pointer-events-none absolute left-3 top-2.5 h-4 w-4 text-secondary-400" />
					<input
						type="search"
						value={filters.search}
						onChange={(e) =>
							setFilters((prev) => ({ ...prev, search: e.target.value }))
						}
						placeholder="Search user, host, or mode"
						className="h-9 w-full rounded-md border border-secondary-300 bg-white pl-9 pr-3 text-sm text-secondary-900 outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-secondary-700 dark:bg-secondary-900 dark:text-white"
					/>
				</label>
				<select
					value={filters.protocol}
					onChange={(e) =>
						setFilters((prev) => ({ ...prev, protocol: e.target.value }))
					}
					className="h-9 rounded-md border border-secondary-300 bg-white px-3 text-sm text-secondary-900 outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-secondary-700 dark:bg-secondary-900 dark:text-white"
				>
					<option value="">All protocols</option>
					<option value="ssh">SSH</option>
					<option value="rdp">RDP</option>
				</select>
				<select
					value={filters.status}
					onChange={(e) =>
						setFilters((prev) => ({ ...prev, status: e.target.value }))
					}
					className="h-9 rounded-md border border-secondary-300 bg-white px-3 text-sm text-secondary-900 outline-none focus:border-primary-500 focus:ring-2 focus:ring-primary-500/20 dark:border-secondary-700 dark:bg-secondary-900 dark:text-white"
				>
					<option value="">All statuses</option>
					<option value="connecting">Connecting</option>
					<option value="connected">Connected</option>
					<option value="closed">Closed</option>
					<option value="failed">Failed</option>
					<option value="timeout">Timeout</option>
					<option value="agent_disconnected">Agent disconnected</option>
				</select>
			</div>

			<div className="overflow-hidden rounded-lg border border-secondary-200 bg-white dark:border-secondary-800 dark:bg-secondary-900">
				<div className="overflow-x-auto">
					<table className="min-w-[900px] w-full table-fixed divide-y divide-secondary-200 dark:divide-secondary-800">
						<colgroup>
							<col className="w-[14%]" />
							<col className="w-[21%]" />
							<col className="w-[24%]" />
							<col className="w-[20%]" />
							<col className="w-[9%]" />
							<col className="w-[12%]" />
						</colgroup>
						<thead className="bg-secondary-50 dark:bg-secondary-950">
							<tr>
								<th className="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-secondary-500">
									Session
								</th>
								<th className="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-secondary-500">
									Host
								</th>
								<th className="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-secondary-500">
									Status
								</th>
								<th className="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-secondary-500">
									Started
								</th>
								<th className="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-secondary-500">
									Duration
								</th>
								<th className="px-3 py-3 text-left text-xs font-semibold uppercase tracking-wide text-secondary-500">
									Recording
								</th>
							</tr>
						</thead>
						<tbody className="divide-y divide-secondary-100 dark:divide-secondary-800">
							{isLoading ? (
								<tr>
									<td
										colSpan={6}
										className="px-4 py-10 text-center text-sm text-secondary-500"
									>
										Loading sessions...
									</td>
								</tr>
							) : sessions.length === 0 ? (
								<tr>
									<td
										colSpan={6}
										className="px-4 py-10 text-center text-sm text-secondary-500"
									>
										No remote access sessions found.
									</td>
								</tr>
							) : (
								sessions.map((session) => (
									<tr
										key={session.id}
										className="hover:bg-secondary-50 dark:hover:bg-secondary-950"
									>
										<td className="px-3 py-3">
											<div className="flex items-center gap-2">
												<MonitorUp className="h-4 w-4 shrink-0 text-secondary-400" />
												<div className="min-w-0">
													<div className="text-sm font-medium uppercase text-secondary-900 dark:text-white">
														{session.protocol}
													</div>
													<div className="truncate text-xs text-secondary-500">
														{session.username || session.user_id} ·{" "}
														{session.connection_mode}
													</div>
												</div>
											</div>
										</td>
										<td className="px-3 py-3">
											<div className="break-words text-sm font-medium leading-5 text-secondary-900 dark:text-white">
												{session.host_name || session.host_id}
											</div>
											<div className="break-words text-xs text-secondary-500">
												{session.host_hostname || session.host_id}
											</div>
										</td>
										<td className="px-3 py-3">
											<StatusBadge status={session.status} />
											{session.error_message ? (
												<div className="mt-1 line-clamp-2 text-xs text-red-600 dark:text-red-300">
													{session.error_message}
												</div>
											) : null}
										</td>
										<td className="px-3 py-3 text-sm text-secondary-700 dark:text-secondary-300">
											<div className="whitespace-nowrap">
												{formatDateTime(session.started_at)}
											</div>
										</td>
										<td className="px-3 py-3 text-sm font-medium text-secondary-800 dark:text-secondary-200">
											<div className="whitespace-nowrap">
												{formatDuration(sessionDurationSeconds(session, now))}
											</div>
										</td>
										<td className="px-3 py-3 text-sm text-secondary-700 dark:text-secondary-300">
											<div className="flex flex-wrap items-center gap-2">
												<span className="text-xs">
													{session.recording_status?.replaceAll("_", " ") ||
														"not requested"}
												</span>
												{session.protocol === "ssh" &&
												session.connection_mode === "guacd" &&
												session.recording_name &&
												session.status === "closed" ? (
													<button
														type="button"
														onClick={() => setSelectedRecording(session)}
														className="inline-flex h-7 items-center gap-1 rounded-md bg-primary-600 px-2 text-xs font-medium text-white hover:bg-primary-700"
													>
														<Play className="h-3.5 w-3.5" />
														View
													</button>
												) : null}
											</div>
										</td>
									</tr>
								))
							)}
						</tbody>
					</table>
				</div>
			</div>
			{selectedRecording ? (
				<RecordingPlayer
					session={selectedRecording}
					onClose={() => setSelectedRecording(null)}
				/>
			) : null}
		</div>
	);
};

export default RemoteAccessSessions;
