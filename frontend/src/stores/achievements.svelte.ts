import { getPlayerAchievements } from '../lib/api';
import type { Achievement, AchievementID, EvAchievementUnlocked } from '../types';

export interface AchievementToast {
    id: string;
    achievement: Achievement;
}

const TOAST_TTL_MS = 5000;

export function createAchievementsStore() {
    let list = $state<Achievement[]>([]);
    let toasts = $state<AchievementToast[]>([]);
    let loading = $state(false);
    let error = $state<string | null>(null);

    async function load() {
        loading = true;
        error = null;
        try {
            const res = await getPlayerAchievements();
            list = res.achievements;
        } catch (err: any) {
            error = err.message || 'Не удалось загрузить достижения';
        } finally {
            loading = false;
        }
    }

    function findById(id: AchievementID): Achievement | undefined {
        return list.find((a) => a.id === id);
    }

    function hasToastFor(id: AchievementID): boolean {
        return toasts.some((t) => t.achievement.id === id);
    }

    function addToast(achievement: Achievement) {
        if (hasToastFor(achievement.id)) return;

        const toast: AchievementToast = {
            id: `${achievement.id}-${Date.now()}`,
            achievement,
        };
        toasts = [...toasts, toast];
        setTimeout(() => removeToast(toast.id), TOAST_TTL_MS);
    }

    function removeToast(id: string) {
        toasts = toasts.filter((t) => t.id !== id);
    }

    function handleUnlock(ev: EvAchievementUnlocked, currentPlayerUid: string) {
        if (ev.player_uid !== currentPlayerUid) return;

        const existing = findById(ev.achievement_id);
        if (existing) {
            list = list.map((a) =>
                a.id === ev.achievement_id ? { ...a, unlocked: true } : a,
            );
            addToast({ ...existing, unlocked: true });
        } else {
            addToast({
                id: ev.achievement_id,
                name: ev.name,
                description: '',
                unlocked: true,
            });
        }
    }

    function reset() {
        list = [];
        toasts = [];
        loading = false;
        error = null;
    }

    return {
        get list() {
            return list;
        },
        get toasts() {
            return toasts;
        },
        get loading() {
            return loading;
        },
        get error() {
            return error;
        },
        get unlockedCount() {
            return list.filter((a) => a.unlocked).length;
        },
        get totalCount() {
            return list.length;
        },
        load,
        handleUnlock,
        removeToast,
        reset,
    };
}

export const achievements = createAchievementsStore();
