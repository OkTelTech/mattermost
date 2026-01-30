// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import { useSelector } from 'react-redux';
import { Link } from 'react-router-dom';

import { getConfig } from 'mattermost-redux/selectors/entities/general';

import BackButton from 'components/common/back_button';
import Logo from 'components/common/svg_images_components/logo_dark_blue_svg';

import logoOktel from 'images/logo-oktel.png';

import './header.scss';

export type HeaderProps = {
    backButtonURL?: string;
    onBackButtonClick?: React.EventHandler<React.MouseEvent>;
}

const Header = ({ backButtonURL, onBackButtonClick }: HeaderProps) => {
    const { SiteName } = useSelector(getConfig);
    const ariaLabel = SiteName || 'Oktel';

    return (
        <div className='hfroute-header'>
            <div className='header-content'>
                <Link
                    className='header-logo-link'
                    to='/'
                    aria-label={ariaLabel}
                >
                    <img src={logoOktel} alt='OKTEL' className='header-logo-img' />
                </Link>
                {onBackButtonClick && (
                    <BackButton
                        className='header-back-button'
                        url={backButtonURL}
                        onClick={onBackButtonClick}
                    />
                )}
            </div>
        </div>
    );
};

export default Header;
