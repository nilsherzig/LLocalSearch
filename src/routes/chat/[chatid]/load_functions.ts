import type { ChatListItem, LogElement, MetricsResponse } from "$lib/types/types";

export async function fetchChats(): Promise<ChatListItem[]> {
    const res = await fetch(`/api/chats/`);
    const data = await res.json();
    if (data['error']) {
        if (typeof window !== 'undefined') {
            throw new Error(data['error']);
        }
    }
    return data;
}

export async function runMetrics(version: string, model: string): Promise<MetricsResponse> {
    const res = await fetch(`https://lsm.nilsherzig.com/v1`, {
        method: 'POST',
        headers: {
            'Content-Type': 'application/json',
        },
        body: JSON.stringify({
            "version": version,
            "model": model,
        }),
    });
    if (res.status !== 200) {
        throw new Error("Failed to check with metrics server");
    }
    const data = await res.json();
    if (data['error']) {
        if (typeof window !== 'undefined') {
            throw new Error(data['error']);
        }
    }
    return data;
}
export async function fetchHistory(id: string): Promise<LogElement[]> {
    if (!id) {
        return [];
    }
    if (id == "new") {
        return [];
    }
    const res = await fetch(`/api/chat/${id}`);
    const data = await res.json();
    if (data['error']) {
        if (typeof window !== 'undefined') {
            throw new Error(data['error']);
        }
    }
    let parsedData = parseLogArray(data);
    return parsedData;
}

let lastLogitemWasStream = false;
export function parseLogArray(history: LogElement[]) {
    if (!history) {
        return [];
    }
    let logBuffer: LogElement[] = [];
    for (const historyItem of history) {
        if (historyItem.message) {
            historyItem.message = historyItem.message.replaceAll('<|im_end|>', '').replaceAll('<|end_of_turn|>', '');
            if (historyItem.stream) {
                if (lastLogitemWasStream) {
                    logBuffer[logBuffer.length - 1].message += historyItem.message;
                } else {
                    logBuffer.push(historyItem);
                }
                lastLogitemWasStream = true;
            } else {
                lastLogitemWasStream = false;
                logBuffer.push(historyItem);
            }
        }
    }
    return logBuffer;
}
