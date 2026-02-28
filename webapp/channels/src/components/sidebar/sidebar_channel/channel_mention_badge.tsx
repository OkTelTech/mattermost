// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React from 'react';

type Props = {
    unreadMentions: number;
    unreadMsgs?: number;
    hasUrgent?: boolean;
    icon?: React.ReactNode;
    className?: string;
};

export default function ChannelMentionBadge({unreadMentions, unreadMsgs, hasUrgent, icon, className}: Props) {
    const count = unreadMentions > 0 ? unreadMentions : (unreadMsgs ?? 0);
    if (count > 0) {
        return (
            <span
                id='unreadMentions'
                className={classNames({badge: true, urgent: hasUrgent && unreadMentions > 0}, className)}
            >
                {icon}
                <span className='unreadMentions'>
                    {count}
                </span>
            </span>
        );
    }

    return null;
}
