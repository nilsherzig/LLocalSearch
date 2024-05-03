import type { ChatListItem, LogElement } from "$lib/types/types";

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
