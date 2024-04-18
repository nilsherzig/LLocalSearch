import type { LogElement, ChatListItem } from '$lib/types/types';
import type { PageLoad } from './$types';

let lastLogitemWasStream = false;

function parseLogArray(history: LogElement[]) {
    if (!history) {
        return [];
    }
    let logBuffer: LogElement[] = [];
    for (const historyItem of history) {
        if (historyItem.message) {
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

export const load: PageLoad = async ({ fetch, params }) => {

    if (!params.chatid) {
        return { status: 404 };
    }

    const fetchChats: Promise<ChatListItem[]> = fetch(`/api/chats/`)
        .then((res) => {
            return res.json()
        })
        .then((data) => {
            if (data["error"]) {
                if (typeof window !== 'undefined') {
                    throw new Error(data["error"]);
                }
            }
            return data
        }).catch((err) => {
            console.log('err', err);
            if (typeof window !== 'undefined') {
                throw new Error(err);
            }
        });

    if (params.chatid == "new") {
        let emptyChatHistoryPromise = Promise.resolve([]);
        return { item: { emptyChatHistoryPromise, fetchChats } };
    }

    const fetchHistory: Promise<LogElement[]> = fetch(`/api/chat/${params.chatid}`)
        .then((res) => {
            return res.json()
        })
        .then((data) => {
            if (data["error"]) {
                if (typeof window !== 'undefined') {
                    throw new Error(data["error"]);
                }
            }
            data = parseLogArray(data);
            // console.log(data)
            return data
        }).catch((err) => {
            console.log('err', err);
            if (typeof window !== 'undefined') {
                throw new Error(err);
            }
        });

    return { item: { fetchHistory, fetchChats } };
}; 
