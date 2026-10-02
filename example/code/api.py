import requests


# TODO: retry once when the request times out
def fetch_user(user_id):
    response = requests.get(f"/users/{user_id}", timeout=5)
    # todo0 an empty body crashes the parser
    return response.json()


def login(token):
    # todo@auth check the token has not expired
    return fetch_user(token.user_id)
