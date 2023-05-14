#!/usr/bin/python3


import socket
import typing

import yaml

import modules.API.Yandex
import modules.web

URL_GET_EXTERNAL_IPv4 = "http://ipecho.net/plain"
DST_GET_EXTERNAL_IP = "a.root-servers.net"
CONFIG_PATH = "/etc/update-ip.yml"


def detect_ipv4(url=URL_GET_EXTERNAL_IPv4):
    try:
        ip_addr = modules.web.get_url_body(url)
    except Exception as err:
        return (False, f"cannot define IPv4 addr, {err}")

    return (True, ip_addr)


def detect_local_ip(addr_type=socket.AF_INET6, dst_addr=DST_GET_EXTERNAL_IP):
    try:
        s = socket.socket(addr_type, socket.SOCK_DGRAM)
        s.connect((dst_addr, 80))

        src_ip = s.getsockname()[0]
        if src_ip:
            return (True, src_ip)
        return (False, "cannot define IPv6 addr")

    except BaseException as err:
        return (False, str(err))


if __name__ == "__main__":
    config: typing.Dict[str, typing.Any]
    with open(CONFIG_PATH, "r", encoding="utf-8") as fh:
        config = yaml.load(fh, Loader=yaml.BaseLoader)

    dns_obj = modules.API.Yandex.API360(config["organization"], config["domain"], config["token"])
    domains_desc = dns_obj.list_domain()
    if "records" not in domains_desc:
        raise ValueError("cannot get domain description")

    for func_detect_ip in [detect_ipv4, detect_local_ip]:
        record_id = None
        message = None
        record_exists_flag = False

        (status, ip_addr) = func_detect_ip()
        if not status:
            print("error: cannot detect IPaddr, %s" % ip_addr)
            continue
        record_type = "AAAA" if ":" in ip_addr else "A"

        for record_desc in domains_desc["records"]:
            if record_desc["type"] == record_type and record_desc["name"] == config["subdomain"]:
                if record_desc["address"] == ip_addr:
                    record_exists_flag = True
                    break

                record_id = record_desc["recordId"]
                break
        if record_exists_flag:
            continue

        if record_id:
            message = dns_obj.edit_domain(
                **{
                    "record_id": record_id,
                    "address": ip_addr,
                    "name": config["subdomain"],
                    "record_type": record_type,
                    "ttl": config["ttl"],
                }
            )
        else:
            message = dns_obj.add_domain(
                **{
                    "address": ip_addr,
                    "name": config["subdomain"],
                    "record_type": record_type,
                    "ttl": config["ttl"],
                }
            )

        print("message: %s" % message)
