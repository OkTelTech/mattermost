// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useState} from 'react';
import {FormattedMessage, useIntl} from 'react-intl';

import type {Session} from '@mattermost/types/sessions';

import {Client4} from 'mattermost-redux/client';

import ConfirmModal from 'components/confirm_modal';
import LoadingSpinner from 'components/widgets/loading/loading_spinner';

import './device_management.scss';

type DeviceSession = Session & {device_type: 'mobile' | 'desktop'; is_current: boolean};

type Props = {
    userId: string;
};

type State = {
    sessions: DeviceSession[];
    maxMobileDevices: number;
    maxDesktopDevices: number;
    bypassLimit: boolean;
    loading: boolean;
    saving: boolean;
    error: string | null;
    revokeTarget: DeviceSession | null;
    inputMobile: number;
    inputDesktop: number;
};

export default function DeviceManagement({userId}: Props) {
    const intl = useIntl();
    const [state, setState] = useState<State>({
        sessions: [],
        maxMobileDevices: 1,
        maxDesktopDevices: 1,
        bypassLimit: false,
        loading: true,
        saving: false,
        error: null,
        revokeTarget: null,
        inputMobile: 1,
        inputDesktop: 1,
    });

    const load = useCallback(async () => {
        setState((prev) => ({...prev, loading: true, error: null}));
        try {
            const [sessions, limits] = await Promise.all([
                Client4.getUserDeviceSessions(userId),
                Client4.getUserDeviceLimits(userId),
            ]);
            setState((prev) => ({
                ...prev,
                sessions: sessions as DeviceSession[],
                maxMobileDevices: limits.max_mobile_devices,
                maxDesktopDevices: limits.max_desktop_devices,
                bypassLimit: limits.bypass_limit ?? false,
                inputMobile: limits.max_mobile_devices,
                inputDesktop: limits.max_desktop_devices,
                loading: false,
            }));
        } catch {
            setState((prev) => ({
                ...prev,
                loading: false,
                error: intl.formatMessage({
                    id: 'admin.deviceManagement.loadError',
                    defaultMessage: 'Failed to load device sessions.',
                }),
            }));
        }
    }, [userId, intl]);

    useEffect(() => {
        load();
    }, [load]);

    const handleRevoke = useCallback((session: DeviceSession) => {
        setState((prev) => ({...prev, revokeTarget: session}));
    }, []);

    const confirmRevoke = useCallback(async () => {
        const {revokeTarget} = state;
        if (!revokeTarget) {
            return;
        }
        try {
            await Client4.revokeSession(userId, revokeTarget.id);
            setState((prev) => ({
                ...prev,
                revokeTarget: null,
                sessions: prev.sessions.filter((s) => s.id !== revokeTarget.id),
            }));
        } catch {
            setState((prev) => ({
                ...prev,
                revokeTarget: null,
                error: intl.formatMessage({
                    id: 'admin.deviceManagement.revokeError',
                    defaultMessage: 'Failed to revoke session.',
                }),
            }));
        }
    }, [state, userId, intl]);

    const cancelRevoke = useCallback(() => {
        setState((prev) => ({...prev, revokeTarget: null}));
    }, []);

    const handleSaveLimits = useCallback(async () => {
        setState((prev) => ({...prev, saving: true, error: null}));
        try {
            const data = await Client4.updateUserDeviceLimits(userId, state.inputMobile, state.inputDesktop);
            setState((prev) => ({
                ...prev,
                saving: false,
                maxMobileDevices: data.max_mobile_devices,
                maxDesktopDevices: data.max_desktop_devices,
                inputMobile: data.max_mobile_devices,
                inputDesktop: data.max_desktop_devices,
            }));
        } catch {
            setState((prev) => ({
                ...prev,
                saving: false,
                error: intl.formatMessage({
                    id: 'admin.deviceManagement.saveLimitsError',
                    defaultMessage: 'Failed to update device limits.',
                }),
            }));
        }
    }, [userId, state.inputMobile, state.inputDesktop, intl]);

    const mobileSessions = state.sessions.filter((s) => s.device_type === 'mobile');
    const desktopSessions = state.sessions.filter((s) => s.device_type === 'desktop');

    return (
        <div className='DeviceManagement'>
            {state.loading && <LoadingSpinner/>}

            {state.error && (
                <div className='DeviceManagement__error'>
                    {state.error}
                </div>
            )}

            {!state.loading && (
                <>
                    <div className='DeviceManagement__summary'>
                        {state.bypassLimit ? (
                            <span className='DeviceManagement__bypass-badge'>
                                <FormattedMessage
                                    id='admin.deviceManagement.adminBypass'
                                    defaultMessage='System Admin — no device limit'
                                />
                            </span>
                        ) : (
                            <>
                                <span>
                                    <FormattedMessage
                                        id='admin.deviceManagement.mobileCount'
                                        defaultMessage='Mobile: {active}/{limit} active'
                                        values={{active: mobileSessions.length, limit: state.maxMobileDevices}}
                                    />
                                </span>
                                <span className='DeviceManagement__summary-separator'>{'|'}</span>
                                <span>
                                    <FormattedMessage
                                        id='admin.deviceManagement.desktopCount'
                                        defaultMessage='Desktop: {active}/{limit} active'
                                        values={{active: desktopSessions.length, limit: state.maxDesktopDevices}}
                                    />
                                </span>
                            </>
                        )}
                    </div>

                    {state.sessions.length > 0 && (
                        <div className='DeviceManagement__table-wrapper'>
                        <table className='DeviceManagement__table'>
                            <thead>
                                <tr>
                                    <th>
                                        <FormattedMessage
                                            id='admin.deviceManagement.colType'
                                            defaultMessage='Type'
                                        />
                                    </th>
                                    <th>
                                        <FormattedMessage
                                            id='admin.deviceManagement.colPlatform'
                                            defaultMessage='Platform'
                                        />
                                    </th>
                                    <th>
                                        <FormattedMessage
                                            id='admin.deviceManagement.colLastActivity'
                                            defaultMessage='Last Activity'
                                        />
                                    </th>
                                    <th/>
                                </tr>
                            </thead>
                            <tbody>
                                {state.sessions.map((s) => (
                                    <DeviceRow
                                        key={s.id}
                                        session={s}
                                        onRevoke={handleRevoke}
                                        isCurrent={s.is_current}
                                    />
                                ))}
                            </tbody>
                        </table>
                        </div>
                    )}

                    {state.sessions.length === 0 && (
                        <p className='DeviceManagement__empty'>
                            <FormattedMessage
                                id='admin.deviceManagement.noSessions'
                                defaultMessage='No active sessions.'
                            />
                        </p>
                    )}

                    {!state.bypassLimit && <div className='DeviceManagement__limits'>
                        <h4>
                            <FormattedMessage
                                id='admin.deviceManagement.limitsTitle'
                                defaultMessage='Device Limits'
                            />
                        </h4>
                        <div className='DeviceManagement__limits-fields'>
                            <label>
                                <FormattedMessage
                                    id='admin.deviceManagement.maxMobile'
                                    defaultMessage='Max Mobile Devices'
                                />
                                <input
                                    type='number'
                                    min={1}
                                    max={10}
                                    value={state.inputMobile}
                                    onChange={(e) => setState((prev) => ({...prev, inputMobile: parseInt(e.target.value, 10) || 1}))}
                                    className='form-control DeviceManagement__limit-input'
                                />
                            </label>
                            <label>
                                <FormattedMessage
                                    id='admin.deviceManagement.maxDesktop'
                                    defaultMessage='Max Desktop Devices'
                                />
                                <input
                                    type='number'
                                    min={1}
                                    max={10}
                                    value={state.inputDesktop}
                                    onChange={(e) => setState((prev) => ({...prev, inputDesktop: parseInt(e.target.value, 10) || 1}))}
                                    className='form-control DeviceManagement__limit-input'
                                />
                            </label>
                        </div>
                        <div className='DeviceManagement__limits-actions'>
                            <button
                                type='button'
                                className='btn btn-primary'
                                onClick={handleSaveLimits}
                                disabled={state.saving}
                            >
                                {state.saving ? (
                                    <FormattedMessage
                                        id='admin.deviceManagement.saving'
                                        defaultMessage='Saving...'
                                    />
                                ) : (
                                    <FormattedMessage
                                        id='admin.deviceManagement.save'
                                        defaultMessage='Save'
                                    />
                                )}
                            </button>
                        </div>
                    </div>}
                </>
            )}

            {state.revokeTarget && (
                <ConfirmModal
                    show={true}
                    title={intl.formatMessage({
                        id: 'admin.deviceManagement.revokeTitle',
                        defaultMessage: 'Remove Device',
                    })}
                    message={intl.formatMessage({
                        id: 'admin.deviceManagement.revokeMessage',
                        defaultMessage: 'Are you sure you want to remove this device? The user will be logged out from it.',
                    })}
                    confirmButtonText={intl.formatMessage({
                        id: 'admin.deviceManagement.revokeConfirm',
                        defaultMessage: 'Remove',
                    })}
                    onConfirm={confirmRevoke}
                    onCancel={cancelRevoke}
                />
            )}
        </div>
    );
}

type DeviceRowProps = {
    session: DeviceSession;
    onRevoke: (session: DeviceSession) => void;
    isCurrent: boolean;
};

function DeviceRow({session, onRevoke, isCurrent}: DeviceRowProps) {
    const platform = session.props?.platform ?? '—';
    const browser = session.props?.browser ?? '';
    const os = session.props?.os ?? '';

    // Show browser app name without version for brevity (e.g. "Chrome" from "Chrome/120.0")
    const browserName = browser ? browser.split('/')[0] : '';

    // Build a human-readable device label:
    // mobile: "Android · Mattermost" or "iPhone · Mattermost"
    // desktop: "Linux · Chrome"
    const deviceDetail = [os || platform, browserName].filter(Boolean).join(' · ');

    const lastActivity = session.last_activity_at
        ? new Date(session.last_activity_at).toLocaleString()
        : '—';

    return (
        <tr className={`DeviceManagement__row${isCurrent ? ' DeviceManagement__row--current' : ''}`}>
            <td>
                <div className='DeviceManagement__type-cell'>
                    <span className={`DeviceManagement__badge DeviceManagement__badge--${session.device_type}`}>
                        {session.device_type === 'mobile' ? (
                            <FormattedMessage
                                id='admin.deviceManagement.mobile'
                                defaultMessage='Mobile'
                            />
                        ) : (
                            <FormattedMessage
                                id='admin.deviceManagement.desktop'
                                defaultMessage='Desktop'
                            />
                        )}
                    </span>
                    {isCurrent && (
                        <span className='DeviceManagement__current-dot'/>
                    )}
                </div>
            </td>
            <td>
                <div className='DeviceManagement__device-name'>{platform}</div>
                {browserName && (
                    <div className='DeviceManagement__device-detail'>{deviceDetail}</div>
                )}
            </td>
            <td>{lastActivity}</td>
            <td>
                <button
                    type='button'
                    className='btn btn-danger btn-sm'
                    onClick={() => onRevoke(session)}
                    disabled={isCurrent}
                    title={isCurrent ? 'Cannot remove current session' : undefined}
                >
                    <FormattedMessage
                        id='admin.deviceManagement.revoke'
                        defaultMessage='Remove'
                    />
                </button>
            </td>
        </tr>
    );
}
