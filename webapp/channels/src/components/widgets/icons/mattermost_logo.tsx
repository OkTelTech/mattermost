// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import logoImage from 'images/logo-oktel.png';

export default function MattermostLogo(props: React.HTMLAttributes<HTMLSpanElement>) {
    const {formatMessage} = useIntl();
    return (
        <span {...props}>
            <img
                src={logoImage}
                alt={formatMessage({id: 'generic_icons.oktel', defaultMessage: 'Oktel Logo'})}
                style={{width: '100%', height: '100%', objectFit: 'contain'}}
            />
        </span>
    );
}
