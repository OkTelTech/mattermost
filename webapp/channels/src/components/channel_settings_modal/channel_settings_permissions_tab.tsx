// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React, {useCallback, useEffect, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel, ChannelModeration as ChannelModerationData, ChannelModerationPatch} from '@mattermost/types/channels';

import {getChannelModerations as fetchChannelModerations, patchChannelModerations} from 'mattermost-redux/actions/channels';
import {Permissions} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getChannelModerations} from 'mattermost-redux/selectors/entities/channels';

import CheckboxCheckedIcon from 'components/widgets/icons/checkbox_checked_icon';
import type {SaveChangesPanelState} from 'components/widgets/modals/components/save_changes_panel';
import SaveChangesPanel from 'components/widgets/modals/components/save_changes_panel';

import type {GlobalState} from 'types/store';

import './channel_settings_permissions_tab.scss';

type Props = {
    channel: Channel;
    setAreThereUnsavedChanges?: (unsaved: boolean) => void;
    showTabSwitchError?: boolean;
};

const MODERATION_LABELS: Record<string, {titleId: string; titleDefault: string; descId: string; descDefault: string}> = {
    [Permissions.CHANNEL_MODERATED_PERMISSIONS.CREATE_POST]: {
        titleId: 'channel_settings.permissions.create_post',
        titleDefault: 'Create Posts',
        descId: 'channel_settings.permissions.create_post.desc',
        descDefault: 'The ability to create posts in the channel.',
    },
    [Permissions.CHANNEL_MODERATED_PERMISSIONS.CREATE_REACTIONS]: {
        titleId: 'channel_settings.permissions.create_reactions',
        titleDefault: 'Post Reactions',
        descId: 'channel_settings.permissions.create_reactions.desc',
        descDefault: 'The ability to post reactions.',
    },
    [Permissions.CHANNEL_MODERATED_PERMISSIONS.MANAGE_MEMBERS]: {
        titleId: 'channel_settings.permissions.manage_members',
        titleDefault: 'Manage Members',
        descId: 'channel_settings.permissions.manage_members.desc',
        descDefault: 'The ability to add and remove people.',
    },
    [Permissions.CHANNEL_MODERATED_PERMISSIONS.USE_CHANNEL_MENTIONS]: {
        titleId: 'channel_settings.permissions.use_channel_mentions',
        titleDefault: 'Channel Mentions',
        descId: 'channel_settings.permissions.use_channel_mentions.desc',
        descDefault: 'The ability to use @all, @here and @channel.',
    },
    [Permissions.CHANNEL_MODERATED_PERMISSIONS.MANAGE_BOOKMARKS]: {
        titleId: 'channel_settings.permissions.manage_bookmarks',
        titleDefault: 'Manage Bookmarks',
        descId: 'channel_settings.permissions.manage_bookmarks.desc',
        descDefault: 'The ability to add, delete and sort bookmarks.',
    },
};

function ChannelSettingsPermissionsTab({channel, setAreThereUnsavedChanges, showTabSwitchError}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();

    const guestAccountsEnabled = useSelector((state: GlobalState) => getConfig(state)?.EnableGuestAccounts === 'true');
    const serverModerations = useSelector((state: GlobalState) => getChannelModerations(state, channel.id));

    const [localPermissions, setLocalPermissions] = useState<ChannelModerationData[] | null>(null);
    const [formError, setFormError] = useState('');
    const [saveChangesPanelState, setSaveChangesPanelState] = useState<SaveChangesPanelState>();

    // Fetch moderations on mount
    useEffect(() => {
        dispatch(fetchChannelModerations(channel.id));
    }, [dispatch, channel.id]);

    // Sync server state to local state
    useEffect(() => {
        if (serverModerations) {
            setLocalPermissions(serverModerations.map((m) => ({
                ...m,
                roles: {
                    ...m.roles,
                    guests: m.roles.guests ? {...m.roles.guests} : undefined,
                    members: {...m.roles.members},
                },
            })));
        }
    }, [serverModerations]);

    // Track unsaved changes
    const hasUnsavedChanges = useMemo(() => {
        if (!localPermissions || !serverModerations) {
            return false;
        }
        return localPermissions.some((local, i) => {
            const server = serverModerations[i];
            if (!server) {
                return false;
            }
            return (
                local.roles.members.value !== server.roles.members.value ||
                (local.roles.guests?.value ?? false) !== (server.roles.guests?.value ?? false)
            );
        });
    }, [localPermissions, serverModerations]);

    useEffect(() => {
        setAreThereUnsavedChanges?.(hasUnsavedChanges);
    }, [hasUnsavedChanges, setAreThereUnsavedChanges]);

    const handleToggle = useCallback((permissionName: string, role: 'members' | 'guests') => {
        setLocalPermissions((prev) => {
            if (!prev) {
                return prev;
            }
            return prev.map((p) => {
                if (p.name !== permissionName) {
                    return p;
                }
                const updated = {...p, roles: {...p.roles}};
                if (role === 'members') {
                    updated.roles.members = {...p.roles.members, value: !p.roles.members.value};
                } else if (role === 'guests' && p.roles.guests) {
                    updated.roles.guests = {...p.roles.guests, value: !p.roles.guests.value};
                }
                return updated;
            });
        });
        setFormError('');
        setSaveChangesPanelState(undefined);
    }, []);

    const handleSave = useCallback(async () => {
        if (!localPermissions || !serverModerations) {
            return;
        }

        const patches: ChannelModerationPatch[] = [];
        localPermissions.forEach((local, i) => {
            const server = serverModerations[i];
            if (!server) {
                return;
            }
            const membersChanged = local.roles.members.value !== server.roles.members.value;
            const guestsChanged = (local.roles.guests?.value ?? false) !== (server.roles.guests?.value ?? false);
            if (membersChanged || guestsChanged) {
                const patch: ChannelModerationPatch = {
                    name: local.name,
                    roles: {},
                };
                if (membersChanged) {
                    patch.roles.members = local.roles.members.value;
                }
                if (guestsChanged) {
                    patch.roles.guests = local.roles.guests?.value;
                }
                patches.push(patch);
            }
        });

        if (patches.length === 0) {
            return;
        }

        const {error} = await dispatch(patchChannelModerations(channel.id, patches)) as any;
        if (error) {
            setFormError(error.message || formatMessage({id: 'channel_settings.permissions.save_error', defaultMessage: 'Failed to save permissions.'}));
            setSaveChangesPanelState('error');
            return;
        }

        setFormError('');
        setSaveChangesPanelState('saved');
    }, [localPermissions, serverModerations, channel.id, dispatch, formatMessage]);

    const handleCancel = useCallback(() => {
        if (serverModerations) {
            setLocalPermissions(serverModerations.map((m) => ({
                ...m,
                roles: {
                    ...m.roles,
                    guests: m.roles.guests ? {...m.roles.guests} : undefined,
                    members: {...m.roles.members},
                },
            })));
        }
        setFormError('');
        setSaveChangesPanelState(undefined);
    }, [serverModerations]);

    const handleClose = useCallback(() => {
        setSaveChangesPanelState(undefined);
    }, []);

    const hasErrors = Boolean(formError) || Boolean(showTabSwitchError);
    const showSaveChangesPanel = hasUnsavedChanges || saveChangesPanelState === 'saved';

    if (!localPermissions) {
        return null;
    }

    return (
        <div className='ChannelSettingsModal__permissionsTab'>
            <div className='permissions-header'>
                <label className='permissions-header__title'>
                    {formatMessage({id: 'channel_settings.permissions.title', defaultMessage: 'Channel Permissions'})}
                </label>
                <label className='permissions-header__subtitle'>
                    {formatMessage({id: 'channel_settings.permissions.subtitle', defaultMessage: 'Manage the actions available to channel members and guests.'})}
                </label>
            </div>

            <div className='permissions-table-wrapper'>
                <table className='permissions-table'>
                    <thead>
                        <tr>
                            <th className='permission-name-col'>
                                {formatMessage({id: 'channel_settings.permissions.permission', defaultMessage: 'Permission'})}
                            </th>
                            {guestAccountsEnabled && (
                                <th className='permission-check-col'>
                                    {formatMessage({id: 'channel_settings.permissions.guests', defaultMessage: 'Guests'})}
                                </th>
                            )}
                            <th className='permission-check-col'>
                                {formatMessage({id: 'channel_settings.permissions.members', defaultMessage: 'Members'})}
                            </th>
                        </tr>
                    </thead>
                    <tbody>
                        {localPermissions.map((entry) => {
                            const labels = MODERATION_LABELS[entry.name];
                            if (!labels) {
                                return null;
                            }
                            return (
                                <tr key={entry.name}>
                                    <td className='permission-name-col'>
                                        <div className='permission-title'>
                                            {formatMessage({id: labels.titleId, defaultMessage: labels.titleDefault})}
                                        </div>
                                        <div className='permission-desc'>
                                            {formatMessage({id: labels.descId, defaultMessage: labels.descDefault})}
                                        </div>
                                    </td>
                                    {guestAccountsEnabled && (
                                        <td className='permission-check-col'>
                                            {entry.roles.guests && (
                                                <button
                                                    type='button'
                                                    className={classNames('checkbox', {
                                                        checked: entry.roles.guests.value && entry.roles.guests.enabled,
                                                        disabled: !entry.roles.guests.enabled,
                                                    })}
                                                    onClick={() => handleToggle(entry.name, 'guests')}
                                                    disabled={!entry.roles.guests.enabled}
                                                >
                                                    {entry.roles.guests.value && entry.roles.guests.enabled && <CheckboxCheckedIcon/>}
                                                </button>
                                            )}
                                        </td>
                                    )}
                                    <td className='permission-check-col'>
                                        <button
                                            type='button'
                                            className={classNames('checkbox', {
                                                checked: entry.roles.members.value && entry.roles.members.enabled,
                                                disabled: !entry.roles.members.enabled,
                                            })}
                                            onClick={() => handleToggle(entry.name, 'members')}
                                            disabled={!entry.roles.members.enabled}
                                        >
                                            {entry.roles.members.value && entry.roles.members.enabled && <CheckboxCheckedIcon/>}
                                        </button>
                                    </td>
                                </tr>
                            );
                        })}
                    </tbody>
                </table>
            </div>

            {showSaveChangesPanel && (
                <SaveChangesPanel
                    handleSubmit={handleSave}
                    handleCancel={handleCancel}
                    handleClose={handleClose}
                    tabChangeError={hasErrors}
                    state={hasErrors ? 'error' : saveChangesPanelState}
                    customErrorMessage={formError}
                    cancelButtonText={formatMessage({
                        id: 'channel_settings.save_changes_panel.reset',
                        defaultMessage: 'Reset',
                    })}
                />
            )}
        </div>
    );
}

export default ChannelSettingsPermissionsTab;
