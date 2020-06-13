#!/usr/bin/python3


import yaml
import socket
import sys
import modules.API.Yandex
import modules.web


URL_GET_EXTERNAL_IPv4 = "http://ipecho.net/plain"
DST_GET_EXTERNAL_IP = "a.root-servers.net"
CONFIG_PATH = "/etc/update-ip.yml"


def detect_ipv4(url=URL_GET_EXTERNAL_IPv4):
    (status, ip_addr) = modules.web.get_url_body(url)
    if not status:
        return (False, "error: cannot define IPv4 addr")

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
    config = None

    try:
        with open(CONFIG_PATH, "r") as fh:
            config = yaml.load(fh, Loader=yaml.BaseLoader)
    except BaseException as err:
        print(err)
        sys.exit(1)

    dns_obj = modules.API.Yandex.PDD_DNS(config["domain"], config["token"])
    (status, domains_desc) = dns_obj.list_domain()
    if not status or "records" not in domains_desc:
        print("error: cannot get domain description")
        sys.exit(1)

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
            if (
                record_desc["type"] == record_type
                and record_desc["subdomain"] == config["subdomain"]
            ):
                if record_desc["content"] == ip_addr:
                    record_exists_flag = True
                    break

                record_id = record_desc["record_id"]
                break
        if record_exists_flag:
            continue

        if record_id:
            (status, message) = dns_obj.edit_domain(
                {
                    "record_id": record_id,
                    "type": record_type,
                    "subdomain": config["subdomain"],
                    "content": ip_addr,
                    "ttl": config["ttl"],
                }
            )
        else:
            (status, message) = dns_obj.add_domain(
                {
                    "type": record_type,
                    "subdomain": config["subdomain"],
                    "content": ip_addr,
                    "ttl": config["ttl"],
                }
            )

        if not status:
            print("error: %s" % message)

