/*
 * Copyright (C) 2013-2015 Canonical Ltd.
 *
 * This program is free software: you can redistribute it and/or modify it
 * under the terms of the GNU General Public License version 3, as published
 * by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranties of
 * MERCHANTABILITY, SATISFACTORY QUALITY, or FITNESS FOR A PARTICULAR
 * PURPOSE.  See the GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License along
 * with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 * In addition, as a special exception, the copyright holders give
 * permission to link the code of portions of this program with the
 * OpenSSL library under certain conditions as described in each
 * individual source file, and distribute linked combinations
 * including the two.
 * You must obey the GNU General Public License in all respects
 * for all of the code used other than OpenSSL.  If you modify
 * file(s) with this exception, you may extend this exception to your
 * version of the file(s), but you are not obligated to do so.  If you
 * do not wish to do so, delete this exception statement from your
 * version.  If you delete this exception statement from all source
 * files in the program, then also delete it here.
 */

#include "signing.h"

#include <iostream>
#include <QCoreApplication>
#include <QDebug>
#include <QObject>
#include <QString>
#include <QTimer>
#include <QUrlQuery>

namespace UbuntuOne {

SigningExample::SigningExample(QObject *parent, const QString& url) :
    QObject(parent),
    m_url(url),
    m_method("POST")
{
    QObject::connect(&service, SIGNAL(credentialsFound(const Token&)),
                     this, SLOT(handleCredentialsFound(Token)));
    QObject::connect(&service, SIGNAL(credentialsNotFound()),
                     this, SLOT(handleCredentialsNotFound()));
}

SigningExample::~SigningExample(){
}

void SigningExample::setMethod(const QString& method)
{
    m_method = method;
}

void SigningExample::doExample()
{
    service.getCredentials();
}

void SigningExample::handleCredentialsFound(Token token)
{
    qDebug() << "Credentials found, signing url.";
    std::cout << token.signUrl(m_url, m_method).toStdString();
    QCoreApplication::instance()->exit(0);
}

void SigningExample::handleCredentialsNotFound()
{
    qDebug() << "No credentials were found.";
    QCoreApplication::instance()->exit(1);
}


} // namespace UbuntuOne


int main(int argc, char *argv[])
{
    QCoreApplication a(argc, argv);
    if (argc < 2) {
        return 2;
    }
    UbuntuOne::SigningExample *example = new UbuntuOne::SigningExample(&a, argv[1]);
    if (argc == 3) {
        example->setMethod(argv[2]);
    }
    QObject::connect(example, SIGNAL(finished()), &a, SLOT(quit()));
    QTimer::singleShot(0, example, SLOT(doExample()));
    return a.exec();
}


